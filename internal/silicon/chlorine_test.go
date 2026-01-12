package silicon_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
	_ "modernc.org/sqlite"
)

func TestChlorineWorker_creates_attempt_and_transitions(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-chlorine-1"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	// set to chlorine phase
	if err := s.UpdateTaskPhaseAndStatus(taskID, "chlorine", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartChlorineWorker(ctx, s, td, 10*time.Millisecond)
	defer cancelFn()

	// wait for worker to do its job
	deadline := time.Now().Add(5 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'chlorine'`, taskID)
		if err := row.Scan(&attemptID, &artifactsDir); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if attemptID == 0 {
		t.Fatalf("no attempt created")
	}

	// check attempt status ok
	var status string
	row := db.QueryRow(`SELECT status FROM attempts WHERE id = ?`, attemptID)
	if err := row.Scan(&status); err != nil {
		t.Fatalf("read attempt: %v", err)
	}
	if status != "ok" {
		t.Fatalf("expected attempt ok, got %s", status)
	}

	// check task transitioned to done/completed
	task, err := s.GetTask(taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Phase != "done" {
		t.Fatalf("expected phase done, got %s", task.Phase)
	}
	if task.Status != "completed" {
		t.Fatalf("expected status completed, got %s", task.Status)
	}

	// check artifacts files exist
	fullDir := filepath.Join(td, artifactsDir)
	if _, err := os.Stat(filepath.Join(fullDir, "result.json")); err != nil {
		t.Fatalf("missing result.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fullDir, "log.txt")); err != nil {
		t.Fatalf("missing log.txt: %v", err)
	}
}

func TestChlorineWorker_idempotent_on_rerun(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-chlorine-2"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	// set to chlorine phase
	if err := s.UpdateTaskPhaseAndStatus(taskID, "chlorine", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartChlorineWorker(ctx, s, td, 10*time.Millisecond)
	defer cancelFn()

	// wait for worker to do its job
	deadline := time.Now().Add(5 * time.Second)
	var attemptID int64
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id FROM attempts WHERE task_id = ? AND role = 'chlorine'`, taskID)
		if err := row.Scan(&attemptID); err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if attemptID == 0 {
		t.Fatalf("no attempt created")
	}

	// stop worker and run another worker instance (simulating rerun)
	cancelFn()

	// start another worker
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	cancelFn2 := silicon.StartChlorineWorker(ctx2, s, td, 10*time.Millisecond)
	defer cancelFn2()

	// ensure no new attempt is created
	time.Sleep(500 * time.Millisecond)
	rows, err := db.Query(`SELECT id FROM attempts WHERE task_id = ? AND role = 'chlorine'`, taskID)
	if err != nil {
		t.Fatalf("query attempts: %v", err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	if count != 1 {
		t.Fatalf("expected 1 attempt after rerun, got %d", count)
	}
}
