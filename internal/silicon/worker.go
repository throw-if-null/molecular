package silicon

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/throw-if-null/molecular/internal/lithium"
)

// StartLithiumWorker starts a background goroutine that polls for tasks in phase 'lithium'
// and runs the lithium Runner. It returns a cancel func to stop the worker.
// interval controls the worker polling interval. If zero, defaults to 1s.
func StartLithiumWorker(ctx context.Context, s Store, repoRoot string, exe lithium.ExecRunner, interval time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		if interval <= 0 {
			interval = 1 * time.Second
		}
		ticker := time.NewTicker(interval)
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
						// process one task: create an attempt, ensure worktree and write lithium artifacts
						cfg := lithium.Config{
							RepoRoot:      repoRoot,
							TaskID:        t.TaskID,
							WorktreePath:  t.WorktreePath,
							ArtifactsRoot: t.ArtifactsRoot,
						}
						r := lithium.NewRunner(cfg, exe)

						// create attempt record (role = 'lithium')
						attemptID, artifactsDir, _, startedAt, err := s.CreateAttempt(t.TaskID, "lithium")
						if err != nil {
							// can't create attempt, mark task failed
							_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "lithium", "failed")
							continue
						}

						// ensure attempt artifacts directory exists under repoRoot
						fullDir := filepath.Join(repoRoot, artifactsDir)
						_ = os.MkdirAll(fullDir, 0o755)

						// ensure worktree (still idempotent). Capture any output in attempt log.
						wtPath, err := r.EnsureWorktree(ctx)
						if err != nil {
							// write meta, result and log files to attempt dir and mark attempt failed
							meta := map[string]interface{}{
								"task_id":    t.TaskID,
								"attempt_id": attemptID,
								"role":       "lithium",
								"status":     "failed",
								"started_at": startedAt,
							}
							if mb, jerr := json.Marshal(meta); jerr == nil {
								_ = os.WriteFile(filepath.Join(fullDir, "meta.json"), mb, 0o644)
							}
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"lithium"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(err.Error()+"\n"), 0o644)
							_ = s.UpdateAttemptStatus(attemptID, "failed", err.Error())
							_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "lithium", "failed")
							continue
						}

						// run optional lithium hook under .molecular/lithium.sh
						hookOut := ""
						hookErr := error(nil)
						hookPath := filepath.Join(repoRoot, ".molecular", "lithium.sh")
						// only run on unix-like systems when executable
						if fi, err := os.Stat(hookPath); err == nil {
							if runtime.GOOS == "windows" {
								hookOut = "skipped lithium.sh on windows\n"
							} else if fi.Mode()&0111 == 0 {
								hookOut = "lithium.sh exists but not executable, skipping\n"
							} else {
								cmd := exec.CommandContext(ctx, hookPath)
								if wtPath != "" {
									cmd.Dir = wtPath
								}
								out, err := cmd.CombinedOutput()
								hookOut = string(out)
								hookErr = err
							}
						}

						// write meta, result and log indicating success or hook failure
						meta := map[string]interface{}{
							"task_id":       t.TaskID,
							"attempt_id":    attemptID,
							"role":          "lithium",
							"status":        "ok",
							"started_at":    startedAt,
							"worktree_path": wtPath,
						}
						if mb, jerr := json.Marshal(meta); jerr == nil {
							_ = os.WriteFile(filepath.Join(fullDir, "meta.json"), mb, 0o644)
						}
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"ok","role":"lithium"}`), 0o644)
						// write a log entry including worktree ensured and any hook output
						logContent := "worktree ensured\n" + hookOut
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(logContent), 0o644)

						if hookErr != nil {
							// hook failed -> mark attempt and task failed
							_ = s.UpdateAttemptStatus(attemptID, "failed", hookErr.Error())
							_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "lithium", "failed")
							continue
						}

						// mark attempt ok
						_ = s.UpdateAttemptStatus(attemptID, "ok", "")
						// transition phase to carbon (keep status running)
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
// interval controls the worker polling interval. If zero, defaults to 1s.
func StartCarbonWorker(ctx context.Context, s Store, repoRoot string, interval time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		if interval <= 0 {
			interval = 1 * time.Second
		}
		ticker := time.NewTicker(interval)
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
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "carbon")
						if err != nil {
							continue
						}
						// ensure dir exists under repoRoot
						fullDir := filepath.Join(repoRoot, artifactsDir)
						_ = os.MkdirAll(fullDir, 0o755)
						// write meta, placeholder result and log
						meta := map[string]interface{}{
							"task_id":     t.TaskID,
							"attempt_id":  attemptID,
							"role":        "carbon",
							"attempt_num": attemptNum,
							"status":      "running",
							"started_at":  startedAt,
						}
						if mb, err := json.Marshal(meta); err == nil {
							_ = os.WriteFile(filepath.Join(fullDir, "meta.json"), mb, 0o644)
						}
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"summary":"stub","complexity":"unknown","role":"carbon","status":"running"}`), 0o644)
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("carbon stub run\n"), 0o644)
						// simulate transient failure deterministically based on prompt
						if strings.Contains(t.Prompt, "carbon-fail") {
							newCount, err := s.IncrementCarbonRetries(t.TaskID)
							if err != nil {
								_ = s.UpdateAttemptStatus(attemptID, "failed", "increment retry failed")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "failed")
								continue
							}
							_ = s.UpdateAttemptStatus(attemptID, "failed", "transient failure")
							if newCount >= t.CarbonBudget {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "failed")
							} else {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "running")
							}
							continue
						}

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
// interval controls the worker polling interval. If zero, defaults to 1s.
func StartHeliumWorker(ctx context.Context, s Store, repoRoot string, interval time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		if interval <= 0 {
			interval = 1 * time.Second
		}
		ticker := time.NewTicker(interval)
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
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "helium")
						if err != nil {
							continue
						}
						// ensure dir exists under repoRoot
						fullDir := filepath.Join(repoRoot, artifactsDir)
						_ = os.MkdirAll(fullDir, 0o755)
						// write meta for helium attempt
						meta := map[string]interface{}{
							"task_id":     t.TaskID,
							"attempt_id":  attemptID,
							"role":        "helium",
							"attempt_num": attemptNum,
							"status":      "running",
							"started_at":  startedAt,
						}
						if mb, err := json.Marshal(meta); err == nil {
							_ = os.WriteFile(filepath.Join(fullDir, "meta.json"), mb, 0o644)
						}
						// simulate transient failure or request changes deterministically based on prompt
						if strings.Contains(t.Prompt, "helium-fail") {
							newCount, err := s.IncrementHeliumRetries(t.TaskID)
							if err != nil {
								_ = s.UpdateAttemptStatus(attemptID, "failed", "increment retry failed")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "failed")
								continue
							}
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"helium"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("helium transient failure\n"), 0o644)
							_ = s.UpdateAttemptStatus(attemptID, "failed", "transient failure")
							if newCount >= t.HeliumBudget {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "failed")
							} else {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "running")
							}
							continue
						}
						if strings.Contains(t.Prompt, "needs-changes") {
							// helium requests changes -> increment review counter and send back to carbon
							newCount, err := s.IncrementReviewRetries(t.TaskID)
							if err != nil {
								_ = s.UpdateAttemptStatus(attemptID, "failed", "increment review failed")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "failed")
								continue
							}
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"changes_requested","role":"helium"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("helium requested changes\n"), 0o644)
							_ = s.UpdateAttemptStatus(attemptID, "ok", "changes requested")
							if newCount > t.ReviewBudget {
								// exceeded review budget -> fail
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "failed")
							} else {
								// send back to carbon for a full review retry
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "running")
							}
							continue
						}
						// otherwise approved
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"approved","role":"helium"}`), 0o644)
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

// StartChlorineWorker starts a background goroutine that polls for tasks in phase 'chlorine'
// and runs a stubbed chlorine finisher. It creates attempt records and writes
// placeholder artifacts (`final_summary.json`, `log.txt`) under the attempt artifacts dir.
// After a successful stub run the task is transitioned to a terminal state
// (phase 'done', status 'completed'). The worker is idempotent: it only acts on
// tasks with status 'running'.
func StartChlorineWorker(ctx context.Context, s Store, repoRoot string, interval time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		if interval <= 0 {
			interval = 1 * time.Second
		}
		ticker := time.NewTicker(interval)
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
					if t.Phase == "chlorine" && t.Status == "running" {
						// create attempt
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "chlorine")
						if err != nil {
							continue
						}
						fullDir := filepath.Join(repoRoot, artifactsDir)
						_ = os.MkdirAll(fullDir, 0o755)
						meta := map[string]interface{}{
							"task_id":     t.TaskID,
							"attempt_id":  attemptID,
							"role":        "chlorine",
							"attempt_num": attemptNum,
							"status":      "running",
							"started_at":  startedAt,
						}
						if mb, err := json.Marshal(meta); err == nil {
							_ = os.WriteFile(filepath.Join(fullDir, "meta.json"), mb, 0o644)
						}
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"completed","note":"stub","role":"chlorine"}`), 0o644)
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("chlorine stub run\n"), 0o644)
						// mark attempt ok
						_ = s.UpdateAttemptStatus(attemptID, "ok", "")
						// transition task to terminal state
						_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "done", "completed")
					}
				}
			}
		}
	}()
	return cancel
}
