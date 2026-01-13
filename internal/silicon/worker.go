package silicon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
// and runs the configured carbon command in the task worktree. It writes artifacts
// (result.json, log.txt) under the attempt artifacts dir. After a successful run
// the task is transitioned to phase 'helium'. It accepts a CommandRunner which
// abstracts execution for tests. interval controls the worker polling interval. If
// zero, defaults to 1s.
func StartCarbonWorker(ctx context.Context, s Store, repoRoot string, runner CommandRunner, carbonCmd []string, interval time.Duration) context.CancelFunc {
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
						crashNote := ""
						if prev, perr := s.GetLatestAttemptByRole(t.TaskID, "carbon"); perr == nil {
							if strings.Contains(prev.ErrorSummary, "crash recovery") {
								crashNote = "previous run crashed; continue from artifacts\n"
							}
						}
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "carbon")
						if err != nil {
							continue
						}
						if cancelled, cerr := s.IsTaskCancelled(t.TaskID); cerr == nil && cancelled {
							fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
							if ferr != nil {
								_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
								continue
							}
							_ = os.MkdirAll(fullDir, 0o755)
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"carbon"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte(crashNote+"cancelled\n"), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
							continue
						}
						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
							continue
						}
						_ = os.MkdirAll(fullDir, 0o755)
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
						logf, lerr := os.Create(filepath.Join(fullDir, "log.txt"))
						if lerr != nil {
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", lerr.Error())
							updateTaskPhaseWithRetries(s, t.TaskID, "carbon", "failed")
							continue
						}
						_, _ = logf.WriteString("command: ")
						_, _ = logf.WriteString(strings.Join(carbonCmd, " ") + "\n")
						_, _ = logf.WriteString("workdir: " + t.WorktreePath + "\n")
						_, _ = logf.WriteString("started_at: " + startedAt + "\n")
						if crashNote != "" {
							_, _ = logf.WriteString(crashNote)
						}
						wtFull := ""
						if t.WorktreePath != "" {
							if p, perr := paths.SafeJoin(repoRoot, t.WorktreePath); perr == nil {
								wtFull = p
							}
						}
						if wtFull == "" {
							_, _ = logf.WriteString("missing worktree\n")
							_ = logf.Close()
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"carbon","exit_code":-1}`), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", "missing worktree")
							updateTaskPhaseWithRetries(s, t.TaskID, "carbon", "failed")
							continue
						}
						// run attempt-scoped so the cancel endpoint can interrupt this run
						attemptCtx, attemptCancel := context.WithCancel(ctx)
						RegisterAttemptCanceler(t.TaskID, attemptCancel)
						defer UnregisterAttemptCanceler(t.TaskID)
						defer attemptCancel()

						ec, err := runner.Run(attemptCtx, wtFull, carbonCmd, nil, logf, logf)
						finishedAt := time.Now().UTC().Format(time.RFC3339Nano)
						_, _ = logf.WriteString("finished_at: " + finishedAt + "\n")
						_, _ = logf.WriteString("exit_code: ")
						_, _ = logf.WriteString(fmt.Sprintf("%d\n", ec))
						_ = logf.Close()
						resObj := map[string]interface{}{"role": "carbon"}
						if err != nil {
							if errors.Is(err, context.Canceled) {
								resObj["status"] = "cancelled"
								resObj["exit_code"] = ec
								mb, _ := json.Marshal(resObj)
								_ = os.WriteFile(filepath.Join(fullDir, "result.json"), mb, 0o644)
								_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
								continue
							}
							resObj["status"] = "failed"
							resObj["exit_code"] = ec
							mb, _ := json.Marshal(resObj)
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), mb, 0o644)
							newCount, uerr := s.UpdateAttemptStatus(attemptID, "failed", err.Error())
							if uerr != nil {
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
						resObj["status"] = "succeeded"
						resObj["exit_code"] = ec
						mb, _ := json.Marshal(resObj)
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), mb, 0o644)
						_, _ = s.UpdateAttemptStatus(attemptID, "ok", "")
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

func StartHeliumWorker(ctx context.Context, s Store, repoRoot string, runner CommandRunner, heliumCmd []string, interval time.Duration) context.CancelFunc {
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
						crashNote := ""
						if prev, perr := s.GetLatestAttemptByRole(t.TaskID, "helium"); perr == nil {
							if strings.Contains(prev.ErrorSummary, "crash recovery") {
								crashNote = "previous run crashed; continue from artifacts\n"
							}
						}
						attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "helium")
						if err != nil {
							continue
						}
						if cancelled, cerr := s.IsTaskCancelled(t.TaskID); cerr == nil && cancelled {
							fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
							if ferr != nil {
								_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
								continue
							}
							_ = os.MkdirAll(fullDir, 0o755)
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"helium"}`), 0o644)
							_ = os.WriteFile(filepath.Join(fullDir, "log.txt"), []byte("cancelled\n"), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
							continue
						}
						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", ferr.Error())
							continue
						}
						_ = os.MkdirAll(fullDir, 0o755)
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
						// run external helium command in the task worktree
						logf, lerr := os.Create(filepath.Join(fullDir, "log.txt"))
						if lerr != nil {
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", lerr.Error())
							updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
							continue
						}
						_, _ = logf.WriteString("command: ")
						_, _ = logf.WriteString(strings.Join(heliumCmd, " ") + "\n")
						_, _ = logf.WriteString("workdir: " + t.WorktreePath + "\n")
						_, _ = logf.WriteString("started_at: " + startedAt + "\n")
						if crashNote != "" {
							_, _ = logf.WriteString(crashNote)
						}
						wtFull := ""
						if t.WorktreePath != "" {
							if p, perr := paths.SafeJoin(repoRoot, t.WorktreePath); perr == nil {
								wtFull = p
							}
						}
						if wtFull == "" {
							_, _ = logf.WriteString("missing worktree\n")
							_ = logf.Close()
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"helium","exit_code":-1}`), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", "missing worktree")
							updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
							continue
						}
						attemptCtx, attemptCancel := context.WithCancel(ctx)
						RegisterAttemptCanceler(t.TaskID, attemptCancel)
						defer UnregisterAttemptCanceler(t.TaskID)
						defer attemptCancel()
						ec, err := runner.Run(attemptCtx, wtFull, heliumCmd, nil, logf, logf)
						finishedAt := time.Now().UTC().Format(time.RFC3339Nano)
						_, _ = logf.WriteString("finished_at: " + finishedAt + "\n")
						_, _ = logf.WriteString("exit_code: ")
						_, _ = logf.WriteString(fmt.Sprintf("%d\n", ec))
						_ = logf.Close()
						outB, _ := os.ReadFile(filepath.Join(fullDir, "log.txt"))
						// attempt to parse first JSON-looking line of stdout/stderr as decision
						decisionObj := map[string]interface{}{}
						lines := bytes.Split(outB, []byte("\n"))
						var first []byte
						for _, L := range lines {
							t := bytes.TrimSpace(L)
							if len(t) == 0 {
								continue
							}
							// pick the first line that looks like JSON (object or array)
							if t[0] == '{' || t[0] == '[' {
								first = t
								break
							}
						}
						parseErr := errors.New("no decision parsed")
						if first != nil {
							parseErr = json.Unmarshal(first, &decisionObj)
						}
						if err != nil {
							if errors.Is(err, context.Canceled) {
								_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"helium"}`), 0o644)
								_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
								continue
							}
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"helium","exit_code":`+fmt.Sprintf("%d", ec)+`}`), 0o644)
							newCount, uerr := s.UpdateAttemptStatus(attemptID, "failed", err.Error())
							if uerr != nil {
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
								continue
							}
							if newCount >= t.HeliumBudget {
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
							} else {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "running")
							}
							continue
						}
						// fallback: try whole output only if we didn't find a JSON-looking line
						if parseErr != nil {
							parseErr = json.Unmarshal(bytes.TrimSpace(outB), &decisionObj)
						}
						if parseErr != nil {
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", "helium: invalid decision JSON")
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"helium","note":"invalid decision"}`), 0o644)
							updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
							continue
						}

						if mb, merr := json.Marshal(decisionObj); merr == nil {
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), mb, 0o644)
						}
						dec, _ := decisionObj["decision"].(string)
						switch dec {
						case "approved":
							_, _ = s.UpdateAttemptStatus(attemptID, "ok", "approved")
							_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "chlorine", "running")
						case "changes_requested":
							newCount, cerr := s.IncrementReviewRetries(t.TaskID)
							if cerr != nil {
								_, _ = s.UpdateAttemptStatus(attemptID, "failed", "increment review failed")
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
								continue
							}
							_, _ = s.UpdateAttemptStatus(attemptID, "ok", "changes requested")
							if newCount > t.ReviewBudget {
								updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
							} else {
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "carbon", "running")
							}
						case "rejected":
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", "rejected")
							_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "helium", "failed")
						default:
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", "unknown decision")
							updateTaskPhaseWithRetries(s, t.TaskID, "helium", "failed")
						}
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
