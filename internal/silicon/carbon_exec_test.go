package silicon_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/silicon"
)

type fakeRunner struct {
	out   string
	err   error
	delay time.Duration
}

func (f *fakeRunner) Run(ctx context.Context, dir string, argv []string, env []string, stdout, stderr io.Writer) (int, error) {
	if f.delay > 0 {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-time.After(f.delay):
		}
	}
	if f.out != "" {
		stdout.Write([]byte(f.out))
	}
	if f.err != nil {
		return 1, f.err
	}
	return 0, nil
}

func TestCarbonWorker_runs_command_and_writes_logs(t *testing.T) {
	s, db, td, cleanup := setupTestStoreWithDB(t)
	defer cleanup()

	taskID := "task-carbon-run"
	_, _, err := s.CreateTaskOrGetExisting(&api.CreateTaskRequest{TaskID: taskID, Prompt: "p"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := s.UpdateTaskPhaseAndStatus(taskID, "carbon", "running"); err != nil {
		t.Fatalf("update phase: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fr := &fakeRunner{out: "hello\n", delay: 10 * time.Millisecond}
	cancelFn := silicon.StartCarbonWorker(ctx, s, td, fr, []string{"fake"}, 10*time.Millisecond)
	defer cancelFn()

	deadline := time.Now().Add(3 * time.Second)
	var attemptID int64
	var artifactsDir string
	for time.Now().Before(deadline) {
		row := db.QueryRow(`SELECT id, artifacts_dir FROM attempts WHERE task_id = ? AND role = 'carbon'`, taskID)
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
	b, _ := os.ReadFile(filepath.Join(fullDir, "log.txt"))
	if !strings.Contains(string(b), "hello") {
		t.Fatalf("log missing hello: %s", string(b))
	}
}
