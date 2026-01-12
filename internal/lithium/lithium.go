package lithium

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "time"
)

// Config holds settings for a Lithium run.
type Config struct {
    RepoRoot string
    TaskID   string
    WorktreePath string
    ArtifactsRoot string
    Branch string // desired branch name
}

// Runner performs lithium operations.
type Runner struct {
    cfg Config
    exe ExecRunner
}

func NewRunner(cfg Config, exe ExecRunner) *Runner {
    return &Runner{cfg: cfg, exe: exe}
}

// EnsureWorktree makes sure the worktree exists using `git worktree add -b <branch> <path> <base>`.
// It's idempotent: if path exists, do nothing.
func (r *Runner) EnsureWorktree(ctx context.Context) (string, error) {
    // worktree root under repo
    wt := r.cfg.WorktreePath
    if wt == "" {
        return "", fmt.Errorf("empty worktree path")
    }
    // if exists, return
    if fi, err := os.Stat(wt); err == nil && fi.IsDir() {
        return wt, nil
    }

    // create parent directories
    if err := os.MkdirAll(filepath.Dir(wt), 0o755); err != nil {
        return "", err
    }

    // default base branch: HEAD (let git resolve default branch)
    base := "HEAD"
    branch := r.cfg.Branch
    if branch == "" {
        branch = "molecular/" + r.cfg.TaskID
    }

    // build args: worktree add -b <branch> <path> <base>
    args := []string{"worktree", "add", "-b", branch, wt, base}

    // ensure artifacts dir exists
    logDir := filepath.Join(r.cfg.ArtifactsRoot, "lithium", time.Now().UTC().Format("20060102T150405Z"))
    if err := os.MkdirAll(logDir, 0o755); err != nil {
        return "", err
    }
    // run git
    out, err := r.exe.Run(ctx, r.cfg.RepoRoot, "git", args...)
    // write output to log
    _ = os.WriteFile(filepath.Join(logDir, "log.txt"), []byte(out+"\n"+errString(err)), 0o644)
    if err != nil {
        return "", fmt.Errorf("git worktree add failed: %w", err)
    }
    return wt, nil
}

func errString(err error) string {
    if err == nil {
        return ""
    }
    return err.Error()
}
