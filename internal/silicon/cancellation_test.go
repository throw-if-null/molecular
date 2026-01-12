package silicon_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/lithium"
	"github.com/throw-if-null/molecular/internal/silicon"
	_ "modernc.org/sqlite"
)

type blockingExecRunner struct {
	mu       sync.Mutex
	started  bool
	canceled bool
}

func (r *blockingExecRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	r.mu.Lock()
	r.started = true
	r.mu.Unlock()

	<-ctx.Done()

	r.mu.Lock()
	r.canceled = true
	r.mu.Unlock()

	return "", ctx.Err()
}

func (r *blockingExecRunner) Started() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started
}

func (r *blockingExecRunner) Canceled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.canceled
}

func TestCancellationStopsLithiumAttempt(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	// create a minimal git repo for worktree creation
	if err := os.MkdirAll(td, 0o755); err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	if err := runCmd(td, "git", "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(td, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := runCmd(td, "git", "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runCmd(td, "git", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	taskID := "task-cancel-lithium"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	exe := &blockingExecRunner{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelWorker := silicon.StartLithiumWorker(ctx, s, td, exe, 10*time.Millisecond)
	defer cancelWorker()

	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// wait until the runner has started (meaning we entered the attempt scope)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exe.Started() {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !exe.Started() {
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
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if exe.Canceled() {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !exe.Canceled() {
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
		row := db.QueryRow(`SELECT id, artifacts_dir, status FROM attempts WHERE task_id = ? AND role = 'lithium' ORDER BY id DESC LIMIT 1`, taskID)
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
		if !strContains(string(b), "\"status\":\"cancelled\"") {
			time.Sleep(25 * time.Millisecond)
			continue
		}
		return
	}
	// if we get here, dump current attempt status
	row := db.QueryRow(`SELECT status FROM attempts WHERE task_id = ? AND role = 'lithium' ORDER BY id DESC LIMIT 1`, taskID)
	var st string
	_ = row.Scan(&st)
	t.Fatalf("cancellation not persisted in time; latest attempt status=%q", st)
}

// duplicate helper (keep test self-contained)
func strContains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var _ lithium.ExecRunner = (*blockingExecRunner)(nil)
