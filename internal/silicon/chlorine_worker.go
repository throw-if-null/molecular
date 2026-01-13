package silicon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/throw-if-null/molecular/internal/paths"
	"github.com/throw-if-null/molecular/internal/store"
)

// StartChlorineWorkerWithRunner runs the chlorine worker using the provided
// CommandRunner and chlorineCmd. It polls tasks and creates PRs using the
// configured command. See worker.go for patterns used by other workers.
func StartChlorineWorkerWithRunner(ctx context.Context, s Store, repoRoot string, runner CommandRunner, chlorineCmd []string, interval time.Duration) context.CancelFunc {
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
					if t.Phase != "chlorine" || t.Status != "running" {
						continue
					}

					crashNote := ""
					if prev, perr := s.GetLatestAttemptByRole(t.TaskID, "chlorine"); perr == nil {
						if strings.Contains(prev.ErrorSummary, "crash recovery") {
							crashNote = "previous run crashed; continue from artifacts\n"
						}
					}

					attemptID, artifactsDir, attemptNum, startedAt, err := s.CreateAttempt(t.TaskID, "chlorine")
					if err != nil {
						if !errors.Is(err, store.ErrInProgress) {
							updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
						}
						continue
					}

					// cancelled before we started
					if cancelled, cerr := s.IsTaskCancelled(t.TaskID); cerr == nil && cancelled {
						fullDir, ferr := paths.SafeJoin(repoRoot, artifactsDir)
						if ferr != nil {
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
					if mb, jerr := json.Marshal(meta); jerr == nil {
						_ = os.WriteFile(filepath.Join(fullDir, "meta.json"), mb, 0o644)
					}

					logf, lerr := os.Create(filepath.Join(fullDir, "log.txt"))
					if lerr != nil {
						_, _ = s.UpdateAttemptStatus(attemptID, "failed", lerr.Error())
						updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
						continue
					}
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
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"chlorine","exit_code":-1}`), 0o644)
						_, _ = s.UpdateAttemptStatus(attemptID, "failed", "missing worktree")
						updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
						continue
					}

					attemptCtx, attemptCancel := context.WithCancel(ctx)
					RegisterAttemptCanceler(t.TaskID, attemptCancel)
					defer UnregisterAttemptCanceler(t.TaskID)
					defer attemptCancel()

					// branch checkout sequence
					branchName := "molecular/" + t.TaskID
					_, berr := runner.Run(attemptCtx, wtFull, []string{"git", "checkout", "-b", branchName}, nil, logf, logf)
					if berr != nil {
						if errors.Is(berr, context.Canceled) {
							_ = logf.Close()
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"chlorine"}`), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
							_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
							continue
						}
						_, berr = runner.Run(attemptCtx, wtFull, []string{"git", "checkout", "-B", branchName}, nil, logf, logf)
						if berr != nil {
							if errors.Is(berr, context.Canceled) {
								_ = logf.Close()
								_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"chlorine"}`), 0o644)
								_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
								continue
							}
							_, berr = runner.Run(attemptCtx, wtFull, []string{"git", "checkout", branchName}, nil, logf, logf)
							if berr != nil && errors.Is(berr, context.Canceled) {
								_ = logf.Close()
								_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"chlorine"}`), 0o644)
								_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
								continue
							}
						}
					}
					if berr != nil {
						_, _ = logf.WriteString("branch error: " + berr.Error() + "\n")
						_ = logf.Close()
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"chlorine","error_summary":"branch failed"}`), 0o644)
						_, _ = s.UpdateAttemptStatus(attemptID, "failed", berr.Error())
						updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
						continue
					}

					// check for changes and commit
					var statusOut, statusErr bytes.Buffer
					_, serr := runner.Run(attemptCtx, wtFull, []string{"git", "status", "--porcelain"}, nil, &statusOut, &statusErr)
					if serr != nil && !errors.Is(serr, context.Canceled) {
						_, _ = logf.WriteString("git status error: " + serr.Error() + "\n")
						_ = logf.Close()
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"chlorine","error_summary":"git status failed"}`), 0o644)
						_, _ = s.UpdateAttemptStatus(attemptID, "failed", serr.Error())
						updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
						continue
					}
					if strings.TrimSpace(statusOut.String()) != "" {
						_, _ = logf.WriteString("changes detected, committing\n")
						_, _ = runner.Run(attemptCtx, wtFull, []string{"git", "add", "-A"}, nil, logf, logf)
						_, cerr := runner.Run(attemptCtx, wtFull, []string{"git", "commit", "-m", "molecular: " + t.TaskID}, nil, logf, logf)
						if cerr != nil {
							if errors.Is(cerr, context.Canceled) {
								_ = logf.Close()
								_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"chlorine"}`), 0o644)
								_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
								_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
								continue
							}
							_ = logf.Close()
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"failed","role":"chlorine","error_summary":"git commit failed"}`), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "failed", cerr.Error())
							updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
							continue
						}
					} else {
						_, _ = logf.WriteString("no changes to commit\n")
					}

					// run configured PR command
					var outb, errb bytes.Buffer
					_, rerr := runner.Run(attemptCtx, wtFull, chlorineCmd, nil, &outb, &errb)
					finishedAt := time.Now().UTC().Format(time.RFC3339Nano)
					_, _ = logf.WriteString("finished_at: " + finishedAt + "\n")
					_, _ = logf.WriteString("stdout:\n" + outb.String() + "\n")
					_, _ = logf.WriteString("stderr:\n" + errb.String() + "\n")
					_ = logf.Close()

					if rerr != nil {
						if errors.Is(rerr, context.Canceled) {
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), []byte(`{"status":"cancelled","role":"chlorine"}`), 0o644)
							_, _ = s.UpdateAttemptStatus(attemptID, "cancelled", "cancelled")
							_ = s.UpdateTaskPhaseAndStatus(t.TaskID, t.Phase, "cancelled")
							continue
						}
						resObj := map[string]interface{}{"status": "failed", "role": "chlorine", "error_summary": errb.String()}
						if mb, merr := json.Marshal(resObj); merr == nil {
							_ = os.WriteFile(filepath.Join(fullDir, "result.json"), mb, 0o644)
						}
						_, _ = s.UpdateAttemptStatus(attemptID, "failed", rerr.Error())
						updateTaskPhaseWithRetries(s, t.TaskID, "chlorine", "failed")
						continue
					}

					prURL := ""
					for _, L := range strings.Split(outb.String(), "\n") {
						tln := strings.TrimSpace(L)
						if strings.HasPrefix(tln, "http://") || strings.HasPrefix(tln, "https://") {
							prURL = tln
							break
						}
					}
					resObj := map[string]interface{}{"status": "ok", "role": "chlorine"}
					if prURL != "" {
						resObj["pr_url"] = prURL
					}
					if mb, merr := json.Marshal(resObj); merr == nil {
						_ = os.WriteFile(filepath.Join(fullDir, "result.json"), mb, 0o644)
					}
					_, _ = s.UpdateAttemptStatus(attemptID, "ok", "")
					_ = s.UpdateTaskPhaseAndStatus(t.TaskID, "done", "completed")
				}
				// end for _, t
				// end case <-ticker.C
			}
			// end select
		}
		// end for ticker loop
	}()
	return cancel
}
