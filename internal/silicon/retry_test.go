package silicon_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
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
			if strings.Contains(err.Error(), "database is locked") {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			t.Fatalf("get task: %v", err)
		}
		if task.Status == "failed" {
			var cr int
			if err := db.QueryRow("SELECT carbon_retries FROM tasks WHERE task_id = ?", taskID).Scan(&cr); err != nil {
				t.Fatalf("query carbon_retries: %v", err)
			}
			if cr < task.CarbonBudget {
				t.Fatalf("task failed before exhausting budget: %d < %d", cr, task.CarbonBudget)
			}
			// check attempts count
			var attempts int
			if err := db.QueryRow("SELECT COUNT(*) FROM attempts WHERE task_id = ? AND role = 'carbon'", taskID).Scan(&attempts); err != nil {
				t.Fatalf("count attempts: %v", err)
			}
			if attempts < cr {
				t.Fatalf("attempts (%d) < retries (%d)", attempts, cr)
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
			if strings.Contains(err.Error(), "database is locked") {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			t.Fatalf("get task: %v", err)
		}
		if task.Status == "failed" {
			var hr int
			if err := db.QueryRow("SELECT helium_retries FROM tasks WHERE task_id = ?", taskID).Scan(&hr); err != nil {
				t.Fatalf("query helium_retries: %v", err)
			}
			if hr < task.HeliumBudget {
				t.Fatalf("task failed before exhausting helium budget: %d < %d", hr, task.HeliumBudget)
			}
			var attempts int
			if err := db.QueryRow("SELECT COUNT(*) FROM attempts WHERE task_id = ? AND role = 'helium'", taskID).Scan(&attempts); err != nil {
				t.Fatalf("count helium attempts: %v", err)
			}
			if attempts < hr {
				t.Fatalf("helium attempts (%d) < retries (%d)", attempts, hr)
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
			if strings.Contains(err.Error(), "database is locked") {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			t.Fatalf("get task: %v", err)
		}
		// if helium wrote changes_requested artifact, it will have been an attempt; check review_retries
		if task.Status == "failed" {
			var rr int
			if err := db.QueryRow("SELECT review_retries FROM tasks WHERE task_id = ?", taskID).Scan(&rr); err != nil {
				t.Fatalf("query review_retries: %v", err)
			}
			if rr <= task.ReviewBudget {
				t.Fatalf("task failed before exhausting review budget: %d <= %d", rr, task.ReviewBudget)
			}
			return
		}
		// if it cycles back to carbon, ensure review_retries > 0 and helium writes artifact check exists
		if task.Phase == "carbon" {
			var rr int
			if err := db.QueryRow("SELECT review_retries FROM tasks WHERE task_id = ?", taskID).Scan(&rr); err != nil {
				t.Fatalf("query review_retries: %v", err)
			}
			if rr == 0 {
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
