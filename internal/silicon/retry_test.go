package silicon_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
	"github.com/throw-if-null/molecular/internal/store"
	_ "modernc.org/sqlite"
)

func TestRetrySemantics_CarbonTransient(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-retry-carbon"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "do it carbon-fail"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "carbon", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// fast polling
	cancelFn := silicon.StartCarbonWorker(ctx, s, td, 10*time.Millisecond)
	defer cancelFn()

	// wait until task becomes failed or exceeds budget
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := s.GetTask(taskID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if task.Status == "failed" {
			if task.CarbonRetries < task.CarbonBudget {
				t.Fatalf("task failed before exhausting budget: %d < %d", task.CarbonRetries, task.CarbonBudget)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("deadline exceeded waiting for carbon retries")
}

func TestRetrySemantics_HeliumTransient(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-retry-helium"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "please helium-fail"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "helium", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartHeliumWorker(ctx, s, td, 10*time.Millisecond)
	defer cancelFn()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task, err := s.GetTask(taskID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if task.Status == "failed" {
			if task.HeliumRetries < task.HeliumBudget {
				t.Fatalf("task failed before exhausting helium budget: %d < %d", task.HeliumRetries, task.HeliumBudget)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("deadline exceeded waiting for helium retries")
}

func TestRetrySemantics_ReviewLoop(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-review-loop"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "please needs-changes"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "helium", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cFn := silicon.StartCarbonWorker(ctx, s, td, 10*time.Millisecond)
	hFn := silicon.StartHeliumWorker(ctx, s, td, 10*time.Millisecond)
	defer cFn()
	defer hFn()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		task, err := s.GetTask(taskID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		// if helium wrote changes_requested artifact, it will have been an attempt; check review_retries
		if task.Status == "failed" {
			if task.ReviewRetries <= task.ReviewBudget {
				t.Fatalf("task failed before exhausting review budget: %d <= %d", task.ReviewRetries, task.ReviewBudget)
			}
			return
		}
		// if it cycles back to carbon, ensure review_retries > 0 and helium writes artifact check exists
		if task.Phase == "carbon" {
			if task.ReviewRetries == 0 {
				t.Fatalf("expected review_retries > 0 after helium requested changes")
			}
			// check that helium attempt artifact exists on disk at least once
			// find latest helium attempt dir
			rows, err := db.Query("SELECT artifacts_dir FROM attempts WHERE task_id = ? AND role = 'helium' ORDER BY id DESC LIMIT 1", taskID)
			if err == nil {
				var artifactsDir string
				if rows.Next() {
					_ = rows.Scan(&artifactsDir)
					full := filepath.Join(td, artifactsDir)
					if _, err := os.Stat(filepath.Join(full, "helium_result.json")); err != nil {
						t.Fatalf("helium_result.json missing: %v", err)
					}
					rows.Close()
					return
				}
				rows.Close()
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("deadline exceeded waiting for review loop")
}
