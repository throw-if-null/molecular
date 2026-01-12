package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/throw-if-null/molecular/internal/api"
	_ "modernc.org/sqlite"
)

func setupTestStore(t *testing.T) (*Store, string, func()) {
	t.Helper()
	td, err := os.MkdirTemp("", "molecular-test-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	dbpath := filepath.Join(td, "molecular.db")
	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		os.RemoveAll(td)
		t.Fatalf("open db: %v", err)
	}
	_, _ = db.Exec(`PRAGMA busy_timeout = 5000`)
	s := New(db)
	if err := s.Init(); err != nil {
		db.Close()
		os.RemoveAll(td)
		t.Fatalf("init: %v", err)
	}
	return s, td, func() { db.Close(); os.RemoveAll(td) }
}

func TestReconcileClearsAttempt(t *testing.T) {
	s, td, cleanup := setupTestStore(t)
	defer cleanup()

	// create a task and move it to carbon
	r := &api.CreateTaskRequest{TaskID: "task-1", Prompt: "p"}
	if _, _, err := s.CreateTaskWithBudgets(r, 3, 3, 2); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus("task-1", "carbon", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	// create an in-flight attempt
	attemptID, artifactsDir, _, _, err := s.CreateAttempt("task-1", "carbon")
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	fullDir := filepath.Join(td, artifactsDir)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("inflight\n"), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	// reconcile
	if err := s.ReconcileInFlightAttempts(td); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// attempt should be failed and task current_attempt_id cleared
	a, err := s.GetAttempt("task-1", attemptID)
	if err != nil {
		t.Fatalf("get attempt: %v", err)
	}
	if a.Status != "failed" {
		t.Fatalf("expected attempt failed, got %s", a.Status)
	}
	if a.ErrorSummary == "" || !strings.Contains(a.ErrorSummary, "crash recovery") {
		t.Fatalf("expected crash recovery error summary, got %q", a.ErrorSummary)
	}

	tk, err := s.GetTask("task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if tk.CurrentAttemptID != nil {
		t.Fatalf("expected current_attempt_id cleared, got %v", *tk.CurrentAttemptID)
	}
	if tk.Status != "running" {
		t.Fatalf("expected task still running, got %s", tk.Status)
	}

	// check artifact log prefixed
	b, err := os.ReadFile(filepath.Join(fullDir, "log.txt"))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.HasPrefix(string(b), "crash recovery: silicon restart") {
		t.Fatalf("expected log to start with crash note, got %q", string(b))
	}
}

func TestReconcileIdempotent(t *testing.T) {
	s, td, cleanup := setupTestStore(t)
	defer cleanup()

	r := &api.CreateTaskRequest{TaskID: "task-2", Prompt: "p"}
	if _, _, err := s.CreateTaskWithBudgets(r, 3, 3, 2); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus("task-2", "carbon", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}
	attemptID, artifactsDir, _, _, err := s.CreateAttempt("task-2", "carbon")
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	fullDir := filepath.Join(td, artifactsDir)
	_ = os.MkdirAll(fullDir, 0o755)

	if err := s.ReconcileInFlightAttempts(td); err != nil {
		t.Fatalf("reconcile1: %v", err)
	}
	// second reconcile should be no-op and not error
	if err := s.ReconcileInFlightAttempts(td); err != nil {
		t.Fatalf("reconcile2: %v", err)
	}

	a, err := s.GetAttempt("task-2", attemptID)
	if err != nil {
		t.Fatalf("get attempt: %v", err)
	}
	if a.Status != "failed" {
		t.Fatalf("expected attempt failed, got %s", a.Status)
	}
}
