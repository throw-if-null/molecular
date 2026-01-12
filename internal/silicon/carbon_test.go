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

func setupTestStoreWithDB(t *testing.T) (*store.Store, *sql.DB, string, func()) {
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
	s := store.New(db)
	if err := s.Init(); err != nil {
		db.Close()
		os.RemoveAll(td)
		t.Fatalf("init: %v", err)
	}
	return s, db, td, func() {
		db.Close()
		os.RemoveAll(td)
	}
}

func TestCarbonWorker_creates_attempt_and_transitions(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-carbon-1"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	// set to carbon phase
	if err := s.UpdateTaskPhaseAndStatus(taskID, "carbon", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelFn := silicon.StartCarbonWorker(ctx, s, td, 10*time.Millisecond)
	defer cancelFn()

	// wait for worker to do its job
	deadline := time.Now().Add(5 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'carbon'`, taskID)
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

	// check task transitioned to helium
	task, err := s.GetTask(taskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Phase != "helium" {
		t.Fatalf("expected phase helium, got %s", task.Phase)
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
