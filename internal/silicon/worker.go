package silicon

import (
	"context"
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
