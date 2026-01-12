package silicon_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/lithium"
	"github.com/throw-if-null/molecular/internal/silicon"
	_ "modernc.org/sqlite"
)

func TestLithiumWorker_runs_hook_and_logs_output(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip hook tests on windows")
	}
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	// create a fake repo with git init so worktree creation can run in integration
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

	// write .molecular/lithium.sh
	hookPath := filepath.Join(td, ".molecular", "lithium.sh")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatalf("mkdir hook dir: %v", err)
	}
	hook := []byte("#!/bin/sh\necho hook-runner-output\n")
	if err := os.WriteFile(hookPath, hook, 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	taskID := "task-hook-lithium"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartLithiumWorker(ctx, s, td, &storeRealExecRunner{td}, 10*time.Millisecond)
	defer cancelFn()

	// wait for attempt
	deadline := time.Now().Add(5 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'lithium'`, taskID)
		if err := row.Scan(&attemptID, &artifactsDir); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if attemptID == 0 {
		t.Fatalf("no attempt created")
	}

	fullDir := filepath.Join(td, artifactsDir)
	b, err := os.ReadFile(filepath.Join(fullDir, "log.txt"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(b) == "" || !contains(string(b), "hook-runner-output") {
		t.Fatalf("expected hook output in log, got: %s", string(b))
	}
}

func TestChlorineWorker_runs_hook_and_logs_output(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip hook tests on windows")
	}
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	// write chlorine hook
	hookPath := filepath.Join(td, ".molecular", "chlorine.sh")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		t.Fatalf("mkdir hook dir: %v", err)
	}
	hook := []byte("#!/bin/sh\necho chlorine-hook-output\n")
	if err := os.WriteFile(hookPath, hook, 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	taskID := "task-hook-chlorine"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "chlorine", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartChlorineWorker(ctx, s, td, 10*time.Millisecond)
	defer cancelFn()

	// wait for attempt
	deadline := time.Now().Add(5 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'chlorine'`, taskID)
		if err := row.Scan(&attemptID, &artifactsDir); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if attemptID == 0 {
		t.Fatalf("no attempt created")
	}

	fullDir := filepath.Join(td, artifactsDir)
	b, err := os.ReadFile(filepath.Join(fullDir, "log.txt"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if string(b) == "" || !contains(string(b), "chlorine-hook-output") {
		t.Fatalf("expected hook output in log, got: %s", string(b))
	}
}

// contains is a tiny helper to avoid importing strings in this file
func contains(s, sub string) bool {
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

// storeRealExecRunner is a tiny wrapper that uses the real exec runner but
// ensures commands run in the repo root. We implement the lithium.ExecRunner
// interface by delegating to lithium.RealExecRunner but setting Dir manually.
type storeRealExecRunner struct{ repo string }

func (r *storeRealExecRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	// delegate to real runner but ensure Dir is repo when empty
	rr := &lithium.RealExecRunner{}
	if dir == "" {
		dir = r.repo
	}
	return rr.Run(ctx, dir, name, args...)
}
