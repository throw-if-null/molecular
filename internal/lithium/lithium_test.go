package lithium

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

type FakeExec struct {
	LastDir  string
	LastName string
	LastArgs []string
}

func (f *FakeExec) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	f.LastDir = dir
	f.LastName = name
	f.LastArgs = append([]string{}, args...)
	return "fake-out", nil
}

func TestEnsureWorktree_usesExecRunner(t *testing.T) {
	td, err := os.MkdirTemp("", "molecular-test-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	defer os.RemoveAll(td)

	taskID := "task-1"
	repoRoot := td
	worktreePath := filepath.Join(td, ".molecular", "worktrees", taskID)
	artifactsRoot := filepath.Join(td, ".molecular", "runs", taskID)

	f := &FakeExec{}
	r := NewRunner(Config{RepoRoot: repoRoot, TaskID: taskID, WorktreePath: worktreePath, ArtifactsRoot: artifactsRoot}, f)
	_, err = r.EnsureWorktree(context.Background())
	if err != nil {
		t.Fatalf("EnsureWorktree failed: %v", err)
	}
	if f.LastName != "git" {
		t.Fatalf("expected git run, got %s", f.LastName)
	}
	if len(f.LastArgs) < 1 || f.LastArgs[0] != "worktree" {
		t.Fatalf("expected worktree arg, got %v", f.LastArgs)
	}
}

func TestEnsureWorktree_integration_git(t *testing.T) {
	td, err := os.MkdirTemp("", "molecular-int-")
	if err != nil {
		t.Fatalf("tmpdir: %v", err)
	}
	defer os.RemoveAll(td)

	// init git repo
	if err := runCmd(td, "git", "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	// create initial commit
	if err := os.WriteFile(filepath.Join(td, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := runCmd(td, "git", "add", "README.md"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runCmd(td, "git", "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit: %v", err)
	}

	taskID := "task-int-1"
	worktreePath := filepath.Join(td, ".molecular", "worktrees", taskID)
	artifactsRoot := filepath.Join(td, ".molecular", "runs", taskID)

	r := NewRunner(Config{RepoRoot: td, TaskID: taskID, WorktreePath: worktreePath, ArtifactsRoot: artifactsRoot}, &RealExecRunner{})
	_, err = r.EnsureWorktree(context.Background())
	if err != nil {
		t.Fatalf("EnsureWorktree failed: %v", err)
	}
	// verify worktree dir exists
	if fi, err := os.Stat(worktreePath); err != nil || !fi.IsDir() {
		t.Fatalf("expected worktree dir created, stat err=%v", err)
	}
}
