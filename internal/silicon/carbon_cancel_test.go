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

type blockingCommandRunner struct {
	started  chan struct{}
	canceled chan struct{}
}

func (r *blockingCommandRunner) Run(ctx context.Context, dir string, argv []string, env []string, stdout, stderr io.Writer) (int, error) {
	// signal started
	select {
	case r.started <- struct{}{}:
	default:
	}
	// block until canceled
	<-ctx.Done()
	select {
	case r.canceled <- struct{}{}:
	default:
	}
	return -1, ctx.Err()
}

func TestCarbonAttemptCancel(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	// ensure worktree exists (simulate lithium)
	taskID := "task-cancel-carbon"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "carbon", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}
	if task, terr := s.GetTask(taskID); terr == nil {
		fullWT := filepath.Join(td, task.WorktreePath)
		_ = os.MkdirAll(fullWT, 0o755)
	}

	runner := &blockingCommandRunner{started: make(chan struct{}, 1), canceled: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartCarbonWorker(ctx, s, td, runner, []string{"fake"}, 10*time.Millisecond)
	defer cancelFn()

	// start server
	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// wait until runner started
	deadline := time.Now().Add(5 * time.Second)
	select {
	case <-runner.started:
	case <-time.After(time.Until(deadline)):
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
	deadline = time.Now().Add(5 * time.Second)
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
		row := db.QueryRow(`SELECT id, artifacts_dir, status FROM attempts WHERE task_id = ? AND role = 'carbon' ORDER BY id DESC LIMIT 1`, taskID)
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
	row := db.QueryRow(`SELECT status FROM attempts WHERE task_id = ? AND role = 'carbon' ORDER BY id DESC LIMIT 1`, taskID)
	var st string
	_ = row.Scan(&st)
	t.Fatalf("cancellation not persisted in time; latest attempt status=%q", st)
}
