package store

import (
	"database/sql"
	"testing"

	"github.com/throw-if-null/molecular/internal/api"
	_ "modernc.org/sqlite"
)

// TestUpdateAttemptStatusHelium verifies UpdateAttemptStatus increments
// the helium_retries counter, clears current_attempt_id, and marks the
// attempt finished with the failed status returning the new count.
func TestUpdateAttemptStatusHelium(t *testing.T) {
	// in-memory sqlite
	db, err := sql.Open("sqlite", "file::memory:?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	s := New(db)
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}

	// create task with default budgets
	r := &api.CreateTaskRequest{TaskID: "task-xyz", Prompt: "do x"}
	if _, _, err := s.CreateTaskWithBudgets(r, 3, 3, 2); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// create an attempt with role helium
	attemptID, _, _, _, err := s.CreateAttempt(r.TaskID, "helium")
	if err != nil {
		t.Fatalf("create attempt: %v", err)
	}

	newCount, err := s.UpdateAttemptStatus(attemptID, "failed", "transient failure")
	if err != nil {
		t.Fatalf("update attempt status: %v", err)
	}

	if newCount != 1 {
		t.Fatalf("expected newCount 1, got %d", newCount)
	}

	// verify task helium_retries and current_attempt_id cleared
	task, err := s.GetTask(r.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.HeliumBudget == 0 {
		t.Fatalf("unexpected zero budget")
	}
	// helium_retries is not exposed on api.Task; query directly
	var heliumRetries int
	var currentAttempt sql.NullInt64
	if err := db.QueryRow(`SELECT helium_retries, current_attempt_id FROM tasks WHERE task_id = ?`, r.TaskID).Scan(&heliumRetries, &currentAttempt); err != nil {
		t.Fatalf("query tasks: %v", err)
	}
	if heliumRetries != 1 {
		t.Fatalf("expected helium_retries 1, got %d", heliumRetries)
	}
	if currentAttempt.Valid {
		t.Fatalf("expected current_attempt_id NULL, got %v", currentAttempt.Int64)
	}

	// verify attempt row
	attempt, err := s.GetAttempt(r.TaskID, attemptID)
	if err != nil {
		t.Fatalf("get attempt: %v", err)
	}
	if attempt.Status != "failed" {
		t.Fatalf("expected attempt status failed, got %s", attempt.Status)
	}
	if attempt.FinishedAt == "" {
		t.Fatalf("expected finished_at set")
	}
}
