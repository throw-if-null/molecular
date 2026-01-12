package silicon

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/throw-if-null/molecular/internal/lithium"
	"github.com/throw-if-null/molecular/internal/paths"
	"github.com/throw-if-null/molecular/internal/store"
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
					if t.Phase != "lithium" || t.Status != "running" {
						continue
					}
					crashNote := ""
					if prev, perr := s.GetLatestAttemptByRole(t.TaskID, "lithium"); perr == nil {
						if strings.Contains(prev.ErrorSummary, "crash recovery") {
							crashNote = "previous run crashed; continue from artifacts\n"
						}
					}
					cfg := lithium.Config{
						RepoRoot:      repoRoot,
						TaskID:        t.TaskID,
						WorktreePath:  t.WorktreePath,
						ArtifactsRoot: t.ArtifactsRoot,
					}
					r := lithium.NewRunner(cfg, exe)

					attemptID, artifactsDir, _, startedAt, err := s.CreateAttempt(t.TaskID, "lithium")
					if err != nil {
						if !errors.Is(err, store.ErrInProgress) {
							updateTaskPhaseWithRetries(s, t.TaskID, "lithium", "failed")
						}
						continue
					}

					if cancelled, cerr := s.IsTaskCancelled(t.TaskID); cerr == nil && cancelled {
						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
							// skip attempt if path unsafe
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
							continue
						}

						_ = os.MkdirAll(fullDir, 0o755)
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"lithium"}`), 0o644)
						_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
						_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
						continue
					}

					// run attempt scoped
					func() {
						attemptCtx, attemptCancel := context.WithCancel(ctx)
						RegisterAttemptCanceler(t.TaskID, attemptCancel)
						defer UnregisterAttemptCanceler(t.TaskID)
						defer attemptCancel()

						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
							// skip attempt if path unsafe
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
							updateTaskPhaseWithRetries(s, t.TaskID, "lithium", "failed")
							return
						}
						_ = os.MkdirAll(fullDir, 0o755)

						wtPath, err := r.EnsureWorktree(attemptCtx)
						if err != nil {
							if errors.Is(err, context.Canceled) {
								_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"lithium"}`), 0o644)
								_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"cancelled\n"), 0o644)
								_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
								return
							}
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
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+err.Error()+"\n"), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", err.Error())
							updateTaskPhaseWithRetries(s, t.TaskID, "lithium", "failed")
							return
						}

						hookOut := ""
						hookErr := error(nil)
						hookPath := filepath.Join(repoRoot, ".molecular", "lithium.sh")
						if fi, err := os.Stat(hookPath); err == nil {
							hookOut = "hook found\nmode=" + fi.Mode().String() + "\n"
							if runtime.GOOS == "windows" {
								hookOut += "skipped lithium.sh on windows\n"
							} else if fi.Mode()&0111 == 0 {
								hookOut += "lithium.sh exists but not executable, skipping\n"
							} else {
								cmd := exec.CommandContext(attemptCtx, "/bin/sh", "-x", hookPath)
								if wtPath != "" {
									cmd.Dir = wtPath
								}
								out, err := cmd.CombinedOutput()
								hookOut += string(out)
								if err != nil && errors.Is(err, context.Canceled) {
									_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"lithium"}`), 0o644)
									_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"cancelled\n"+hookOut), 0o644)
									_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
									_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
									return
								}
								hookErr = err
							}
						}

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
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("worktree ensured\n"+hookOut), 0o644)

						if hookErr != nil {
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", hookErr.Error())
							updateTaskPhaseWithRetries(s, t.TaskID, "lithium", "failed")
							return
						}

						_, _ = s.UpdateAttemptStatus(attemptID, "ok", "")
						_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "running")
					}()
				}
			}
		}
	}()
	return cancel
}

// updateTaskPhaseWithRetries attempts to update task phase/status, retrying on
// transient sqlite busy errors. Log a warning if it ultimately fails.
func updateTaskPhaseWithRetries(s Store, taskID, phase, status string) {
	const maxRetries = 5
	for i := 0; i < maxRetries; i++ {
		if err := s.UpdateTaskPhaseAndStatus(taskID, phase, status); err == nil {
			return
		} else if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
			time.Sleep(time.Duration(10*(1<<i)) * time.Millisecond)
			continue
		} else {
			log.Printf("UpdateTaskPhaseAndStatus failed for %s: %v", taskID, err)
			return
		}
	}
	log.Printf("UpdateTaskPhaseAndStatus failed after retries for %s", taskID)
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
						// check previous attempt for crash recovery note
						crashNote := ""
						if prev, perr := s.GetLatestAttemptByRole(t.TaskID, "carbon"); perr == nil {
							if strings.Contains(prev.ErrorSummary, "crash recovery") {
								crashNote = "previous run crashed; continue from artifacts\n"
							}
						}
						// create attempt
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "carbon")
						if err != nil {
							continue
						}
						// if task was cancelled between polling and attempt creation,
						// mark the attempt cancelled and skip doing work.
						if cancelled, cerr := s.IsTaskCancelled(t.TaskID); cerr == nil && cancelled {
							fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
							if ferr != nil {
								// skip attempt if path unsafe
								_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
								continue
							}

							_ = os.MkdirAll(fullDir, 0o755)
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"carbon"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"cancelled\n"), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
							continue
						}
						// ensure dir exists under repoRoot
						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
							// skip attempt if path unsafe
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
							continue
						}

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
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"carbon stub run\n"), 0o644)
						// simulate transient failure deterministically based on prompt
						if strings.Contains(t.Prompt, "carbon-fail") {
							newCount, err := s.UpdateAttemptStatus(attemptID, "failed", "transient failure")
							if err != nil {
								updateTaskPhaseWithRetries(s, t.TaskID, "carbon", "failed")
								continue
							}
							if newCount >= t.CarbonBudget {
								updateTaskPhaseWithRetries(s, t.TaskID, "carbon", "failed")
							} else {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "running")
							}
							continue
						}

						// mark attempt ok
						_, _ = s.UpdateAttemptStatus(attemptID, "ok", "")
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
						// check previous attempt for crash recovery note
						crashNote := ""
						if prev, perr := s.GetLatestAttemptByRole(t.TaskID, "helium"); perr == nil {
							if strings.Contains(prev.ErrorSummary, "crash recovery") {
								crashNote = "previous run crashed; continue from artifacts\n"
							}
						}
						// create attempt
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "helium")
						if err != nil {
							continue
						}
						if cancelled, cerr := s.IsTaskCancelled(t.TaskID); cerr == nil && cancelled {
							fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
							if ferr != nil {
								// skip attempt if path unsafe
								_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
								continue
							}

							_ = os.MkdirAll(fullDir, 0o755)
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"helium"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("cancelled\n"), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
							continue
						}
						// ensure dir exists under repoRoot
						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
							// skip attempt if path unsafe
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
							continue
						}

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
							newCount, err := s.UpdateAttemptStatus(attemptID, "failed", "transient failure")
							if err != nil {
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
								continue
							}
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"helium"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"helium transient failure\n"), 0o644)
							if newCount >= t.HeliumBudget {
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
							} else {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "running")
							}
							continue
						}
						if strings.Contains(t.Prompt, "needs-changes") {
							// helium requests changes -> increment review counter and send back to carbon
							newCount, err := s.IncrementReviewRetries(t.TaskID)
							if err != nil {
								_, _ = s.UpdateAttemptStatus(attemptID, "failed", "increment review failed")
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
								continue
							}
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"changes_requested","role":"helium"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"helium requested changes\n"), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "ok", "changes requested")
							if newCount > t.ReviewBudget {
								// exceeded review budget -> fail
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
							} else {
								// send back to carbon for a full review retry
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "running")
							}
							continue
						}
						// otherwise approved
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"approved","role":"helium"}`), 0o644)
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"helium stub run\n"), 0o644)
						// mark attempt ok
						_, _ = s.UpdateAttemptStatus(attemptID, "ok", "")
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
						// check previous attempt for crash recovery note
						crashNote := ""
						if prev, perr := s.GetLatestAttemptByRole(t.TaskID, "chlorine"); perr == nil {
							if strings.Contains(prev.ErrorSummary, "crash recovery") {
								crashNote = "previous run crashed; continue from artifacts\n"
							}
						}
						// create attempt
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "chlorine")
						if err != nil {
							continue
						}
						if cancelled, cerr := s.IsTaskCancelled(t.TaskID); cerr == nil && cancelled {
							fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
							if ferr != nil {
								// skip attempt if path unsafe
								_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
								continue
							}

							_ = os.MkdirAll(fullDir, 0o755)
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"chlorine"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("cancelled\n"), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
							continue
						}
						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
							// skip attempt if path unsafe
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
							continue
						}

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
						// run optional chlorine hook
						hookOut := ""
						hookErr := error(nil)
						hookPath := filepath.Join(repoRoot, ".molecular", "chlorine.sh")
						if fi, err := os.Stat(hookPath); err == nil {
							if runtime.GOOS == "windows" {
								hookOut = "skipped chlorine.sh on windows\n"
							} else if fi.Mode()&0111 == 0 {
								hookOut = "chlorine.sh exists but not executable, skipping\n"
							} else {
								cmd := exec.CommandContext(ctx, "/bin/sh", "-x", hookPath)
								cmd.Dir = fullDir
								out, err := cmd.CombinedOutput()
								hookOut = string(out)
								hookErr = err
							}
						}
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"completed","note":"stub","role":"chlorine"}`), 0o644)
						_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"chlorine stub run\n"+hookOut), 0o644)
						if hookErr != nil {
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", hookErr.Error())
							updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
							continue
						}
						// mark attempt ok
						_, _ = s.UpdateAttemptStatus(attemptID, "ok", "")
						// transition task to terminal state
						updateTaskPhaseWithRetries(s, t.TaskID, "done", "completed")
					}
				}
			}
		}
	}()
	return cancel
}
