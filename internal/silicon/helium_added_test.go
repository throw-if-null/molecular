package silicon_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
	_ "modernc.org/sqlite"
)

// blockingRunnerHelium is like the one used in carbon_cancel_test: signal when
// started then block until the context is cancelled.
type blockingRunnerHelium struct {
	started  chan struct{}
	canceled chan struct{}
}

func (r *blockingRunnerHelium) Run(ctx context.Context, dir string, argv []string, env []string, stdout, stderr io.Writer) (int, error) {
	select {
	case r.started <- struct{}{}:
	default:
	}
	<-ctx.Done()
	select {
	case r.canceled <- struct{}{}:
	default:
	}
	return -1, ctx.Err()
}

// TestHeliumAttemptCancel ensures the helium worker registers an attempt-scoped
// canceler and that the HTTP cancel endpoint causes the runner to stop and the
// attempt to be marked cancelled.
func TestHeliumAttemptCancel(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	// create task and set to helium
	taskID := "task-cancel-helium"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "helium", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}
	// ensure worktree exists
	if task, terr := s.GetTask(taskID); terr == nil {
		fullWT := filepath.Join(td, task.WorktreePath)
		_ = os.MkdirAll(fullWT, 0o755)
	}

	runner := &blockingRunnerHelium{started: make(chan struct{}, 1), canceled: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartHeliumWorker(ctx, s, td, runner, []string{"fake"}, 10*time.Millisecond)
	defer cancelFn()

	// start server
	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// wait until runner started
	select {
	case <-runner.started:
	case <-time.After(5 * time.Second):
		t.Fatalf("runner never started")
	}

	// cancel via HTTP
	req, _ := http.NewRequest("POST", ts.URL+"/v1/tasks/"+taskID+"/cancel", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("cancel request: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %s", res.Status)
	}

	// ensure runner observed cancellation
	select {
	case <-runner.canceled:
	case <-time.After(5 * time.Second):
		t.Fatalf("runner was not canceled")
	}

	// Wait until the worker persists the cancelled task + attempt.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		task, err := s.GetTask(taskID)
		if err != nil {
			// allow transient SQLITE_BUSY while the worker is writing
			time.Sleep(25 * time.Millisecond)
			continue
		}
		if task.Status != "cancelled" {
			time.Sleep(25 * time.Millisecond)
			continue
		}

		// attempt should be cancelled
		var attemptID int64
		var artifactsDir string
		var status string
		row := db.QueryRow(`SELECT id, artifacts_dir, status FROM attempts WHERE task_id = ? AND role = 'helium' ORDER BY id DESC LIMIT 1`, taskID)
		if err := row.Scan(&attemptID, &artifactsDir, &status); err != nil {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		if status != "cancelled" {
			time.Sleep(25 * time.Millisecond)
			continue
		}

		b, err := os.ReadFile(filepath.Join(td, artifactsDir, "result.json"))
		if err != nil {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		if !strContains(string(b), `"status":"cancelled"`) {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		return
	}
	row := db.QueryRow(`SELECT status FROM attempts WHERE task_id = ? AND role = 'helium' ORDER BY id DESC LIMIT 1`, taskID)
	var st string
	_ = row.Scan(&st)
	t.Fatalf("cancellation not persisted in time; latest attempt status=%q", st)
}

// rejectRunner writes a rejected decision to stdout/stderr and returns success.
type rejectRunner struct{}

func (r *rejectRunner) Run(ctx context.Context, dir string, argv []string, env []string, stdout, stderr io.Writer) (int, error) {
	stdout.Write([]byte("{\"decision\":\"rejected\"}\n"))
	return 0, nil
}

// TestHeliumRejectedDecision verifies that a rejected decision marks the
// attempt as failed and the task phase/status transitions to failed.
func TestHeliumRejectedDecision(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-helium-rejected"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "helium", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}
	// ensure worktree exists (simulate lithium)
	if task, terr := s.GetTask(taskID); terr == nil {
		fullWT := filepath.Join(td, task.WorktreePath)
		_ = os.MkdirAll(fullWT, 0o755)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartHeliumWorker(ctx, s, td, &rejectRunner{}, []string{"unused"}, 10*time.Millisecond)
	defer cancelFn()

	// wait for an attempt to be created and marked failed
	deadline := time.Now().Add(5 * time.Second)
	var attemptID int64
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id FROM attempts WHERE task_id = ? AND role = 'helium'`, taskID)
		if err := row.Scan(&attemptID); err == nil {
			// check attempt status
			row2 := db.QueryRow(`SELECT status FROM attempts WHERE id = ?`, attemptID)
			var status string
			if err := row2.Scan(&status); err == nil {
				if status == "failed" {
					// ensure task is marked failed
					task, terr := s.GetTask(taskID)
					if terr != nil {
						t.Fatalf("get task: %v", terr)
					}
					if task.Status != "failed" {
						t.Fatalf("expected task status failed, got %s", task.Status)
					}
					// check result.json contains decision
					var artifactsDir string
					row3 := db.QueryRow(`SELECT artifacts_dir FROM attempts WHERE id = ?`, attemptID)
					if err := row3.Scan(&artifactsDir); err == nil {
						b, _ := os.ReadFile(filepath.Join(td, artifactsDir, "result.json"))
						if !strContains(string(b), `"decision":"rejected"`) {
							t.Fatalf("expected rejected decision in result.json: %s", string(b))
						}
					}
					return
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("deadline exceeded waiting for rejected decision handling")
}
