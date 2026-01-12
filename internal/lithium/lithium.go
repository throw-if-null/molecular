package lithium

import (
	"context"
	"fmt"
	"github.com/throw-if-null/molecular/internal/paths"
	"os"
	"path/filepath"
)

// Config holds settings for a Lithium run.
type Config struct {
	RepoRoot      string
	TaskID        string
	WorktreePath  string
	ArtifactsRoot string
	Branch        string // desired branch name
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
	// derive and validate worktree path from task id to avoid traversal
	if err := paths.ValidateTaskID(r.cfg.TaskID); err != nil {
		return "", fmt.Errorf("invalid task id: %w", err)
	}
	wt, werr := paths.WorktreeDir(r.cfg.TaskID)
	if werr != nil {
		return "", werr
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

	_, err := r.exe.Run(ctx, r.cfg.RepoRoot, "git", args...)
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
