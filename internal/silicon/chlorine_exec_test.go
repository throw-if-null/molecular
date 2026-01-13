package silicon_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
)

// reuse fakeRunner and blockingCommandRunner from carbon tests

func TestChlorineCreatesPR(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-chlorine-pr"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "chlorine", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ensure worktree exists (simulate lithium)
	if task, terr := s.GetTask(taskID); terr == nil {
		fullWT := filepath.Join(td, task.WorktreePath)
		_ = os.MkdirAll(fullWT, 0o755)
	}

	// fake runner that returns a PR URL
	fr := &fakeRunner{out: "https://github.com/org/repo/pull/123\n"}
	cancelFn := silicon.StartChlorineWorkerWithRunner(ctx, s, td, fr, []string{"gh", "pr", "create", "--fill"}, 10*time.Millisecond)
	defer cancelFn()

	deadline := time.Now().Add(3 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'chlorine'`, taskID)
		if err := row.Scan(&attemptID, &artifactsDir); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if attemptID == 0 {
		t.Fatalf("no attempt created")
	}
	fullDir := filepath.Join(td, artifactsDir)
	// wait for result.json
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(fullDir, "result.json")); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := os.ReadFile(filepath.Join(fullDir, "result.json"))
	if !strings.Contains(string(b), "pull/123") {
		t.Fatalf("result missing pr url: %s", string(b))
	}
}

func TestChlorineGhFailure(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-chlorine-fail"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "chlorine", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if task, terr := s.GetTask(taskID); terr == nil {
		fullWT := filepath.Join(td, task.WorktreePath)
		_ = os.MkdirAll(fullWT, 0o755)
	}
	fr := &fakeRunner{out: "", err: io.ErrUnexpectedEOF}
	cancelFn := silicon.StartChlorineWorkerWithRunner(ctx, s, td, fr, []string{"gh", "pr", "create", "--fill"}, 10*time.Millisecond)
	defer cancelFn()

	deadline := time.Now().Add(3 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'chlorine'`, taskID)
		if err := row.Scan(&attemptID, &artifactsDir); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if attemptID == 0 {
		t.Fatalf("no attempt created")
	}
	fullDir := filepath.Join(td, artifactsDir)
	// wait for result.json
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(fullDir, "result.json")); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := os.ReadFile(filepath.Join(fullDir, "result.json"))
	if !strings.Contains(string(b), "failed") {
		t.Fatalf("result not failed: %s", string(b))
	}
}

func TestChlorineAttemptCancel(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-chlorine-cancel"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "chlorine", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if task, terr := s.GetTask(taskID); terr == nil {
		fullWT := filepath.Join(td, task.WorktreePath)
		_ = os.MkdirAll(fullWT, 0o755)
	}

	runner := &blockingCommandRunner{started: make(chan struct{}, 1), canceled: make(chan struct{}, 1)}
	cancelFn := silicon.StartChlorineWorkerWithRunner(ctx, s, td, runner, []string{"gh", "pr", "create", "--fill"}, 10*time.Millisecond)
	defer cancelFn()

	// wait for attempt to start
	deadline := time.Now().Add(3 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'chlorine'`, taskID)
		if err := row.Scan(&attemptID, &artifactsDir); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if attemptID == 0 {
		t.Fatalf("no attempt created")
	}

	// start server
	srv := silicon.NewServer(s, 3, 3, 2)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// wait until runner started
	deadline2 := time.Now().Add(5 * time.Second)
	select {
	case <-runner.started:
	case <-time.After(time.Until(deadline2)):
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

	// wait for result
	fullDir := filepath.Join(td, artifactsDir)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(fullDir, "result.json")); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	b, _ := os.ReadFile(filepath.Join(fullDir, "result.json"))
	if !strings.Contains(string(b), "cancelled") {
		t.Fatalf("result not cancelled: %s", string(b))
	}
}
