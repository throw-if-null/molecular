package silicon

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/throw-if-null/molecular/internal/lithium"
)

// StartLithiumWorker starts a background goroutine that polls for tasks in phase 'lithium'
// and runs the lithium Runner. It returns a cancel func to stop the worker.
func StartLithiumWorker(ctx context.Context, s Store, repoRoot string, exe lithium.ExecRunner) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// list tasks
				tasks, err := s.ListTasks(0)
				if err != nil {
					continue
				}
				for _, t := range tasks {
					if t.Phase == "lithium" && t.Status == "running" {
						// process one task
						cfg := lithium.Config{
							RepoRoot:      repoRoot,
							TaskID:        t.TaskID,
							WorktreePath:  t.WorktreePath,
							ArtifactsRoot: t.ArtifactsRoot,
						}
						r := lithium.NewRunner(cfg, exe)
						// mark as in progress maybe already running; ensure idempotent
						_, _ = r.EnsureWorktree(ctx)
						// transition phase to carbon
						_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "running")
					}
				}
			}
		}
	}()
	return cancel
}

// StartCarbonWorker starts a background goroutine that polls for tasks in phase 'carbon'
// and runs a stubbed carbon worker in-process. It creates attempt records and writes
// placeholder artifacts (carbon_result.json, log.txt) under the attempt artifacts dir.
// After a successful stub run the task is transitioned to phase 'helium'.
func StartCarbonWorker(ctx context.Context, s Store, repoRoot string) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tasks, err := s.ListTasks(0)
				if err != nil {
					continue
				}
				for _, t := range tasks {
					if t.Phase == "carbon" && t.Status == "running" {
						// create attempt
						attemptID, artifactsDir, err := s.CreateAttempt(t.TaskID, "carbon")
						if err != nil {
							continue
						}
						// ensure dir exists under repoRoot
						fullDir := filepath.Join(repoRoot, artifactsDir)
						_ = os.MkdirAll(fullDir, 0o755)
						// write placeholder result and log
						_ = os.WriteFile(filepath.Join(fullDir, "carbon_result.json"), []byte(`{"summary":"stub","complexity":"unknown"}`), 0o644)
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("carbon stub run\n"), 0o644)
						// mark attempt ok
						_ = s.UpdateAttemptStatus(attemptID, "ok", "")
						// transition task to helium (keep status running)
						_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "running")
					}
				}
			}
		}
	}()
	return cancel
}

// StartHeliumWorker starts a background goroutine that polls for tasks in phase 'helium'
// and runs a stubbed helium inspector worker. It creates attempt records and writes
// placeholder artifacts (helium_result.json, log.txt) under the attempt artifacts dir.
// After a successful stub run the task is transitioned to phase 'chlorine'.
func StartHeliumWorker(ctx context.Context, s Store, repoRoot string) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				tasks, err := s.ListTasks(0)
				if err != nil {
					continue
				}
				for _, t := range tasks {
					if t.Phase == "helium" && t.Status == "running" {
						// create attempt
						attemptID, artifactsDir, err := s.CreateAttempt(t.TaskID, "helium")
						if err != nil {
							continue
						}
						// ensure dir exists under repoRoot
						fullDir := filepath.Join(repoRoot, artifactsDir)
						_ = os.MkdirAll(fullDir, 0o755)
						// write placeholder result and log. For now always approved.
						_ = os.WriteFile(filepath.Join(fullDir, "helium_result.json"), []byte(`{"status":"approved"}`), 0o644)
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("helium stub run\n"), 0o644)
						// mark attempt ok
						_ = s.UpdateAttemptStatus(attemptID, "ok", "")
						// transition task to chlorine (keep status running)
						_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "chlorine", "running")
					}
				}
			}
		}
	}()
	return cancel
}
