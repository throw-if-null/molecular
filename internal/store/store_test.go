package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/throw-if-null/molecular/internal/api"
	_ "modernc.org/sqlite"
)

func TestInitAndCreateTask(t *testing.T) {
	td, err := os.MkdirTemp("", "molecular-test-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	defer os.RemoveAll(td)

	dbpath := filepath.Join(td, "molecular.db")
	db, err := sql.Open("sqlite", dbpath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := New(db)
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	// Verify tables exist by inserting via API
	r := &api.CreateTaskRequest{TaskID: "task-1", Prompt: "do something"}
	task, existed, err := s.CreateTaskOrGetExisting(r)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if existed {
		t.Fatalf("expected new task, got existed")
	}
	if task.TaskID != r.TaskID {
		t.Fatalf("task id mismatch")
	}
	if task.Prompt != r.Prompt {
		t.Fatalf("prompt mismatch")
	}
	if task.Status == "" || task.Phase == "" {
		t.Fatalf("status/phase not set")
	}
	if task.CreatedAt == "" || task.UpdatedAt == "" {
		t.Fatalf("timestamps not set")
	}
	if task.CarbonBudget <= 0 || task.HeliumBudget <= 0 || task.ReviewBudget <= 0 {
		t.Fatalf("budgets not defaulted")
	}

	// Idempotent: second create returns existing
	task2, existed, err := s.CreateTaskOrGetExisting(r)
	if err != nil {
		t.Fatalf("create2: %v", err)
	}
	if !existed {
		t.Fatalf("expected existed on second create")
	}
	if task2.TaskID != task.TaskID {
		t.Fatalf("ids mismatch on second create")
	}
}
