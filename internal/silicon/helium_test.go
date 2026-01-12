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

func TestHeliumWorker_creates_attempt_and_transitions(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-helium-1"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	// set to helium phase
	if err := s.UpdateTaskPhaseAndStatus(taskID, "helium", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartHeliumWorker(ctx, s, td, 10*time.Millisecond)
	defer cancelFn()

	// wait for worker to do its job
	deadline := time.Now().Add(5 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'helium'`, taskID)
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

	// check task transitioned to chlorine
	task, err := s.GetTask(taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Phase != "chlorine" {
		t.Fatalf("expected phase chlorine, got %s", task.Phase)
	}

	// check artifacts files exist
	fullDir := filepath.Join(td, artifactsDir)
	if _, err := os.Stat(filepath.Join(fullDir, "helium_result.json")); err != nil {
		t.Fatalf("missing helium_result.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fullDir, "log.txt")); err != nil {
		t.Fatalf("missing log.txt: %v", err)
	}
}
