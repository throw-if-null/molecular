package silicon

import (
	"context"
	"io"
	"os/exec"
	"syscall"
)

// CommandRunner abstracts running external commands so tests can inject fakes.
type CommandRunner interface {
	Run(ctx context.Context, dir string, argv []string, env []string, stdout, stderr io.Writer) (exitCode int, err error)
}

// RealCommandRunner runs commands using os/exec.
type RealCommandRunner struct{}

func (r *RealCommandRunner) Run(ctx context.Context, dir string, argv []string, env []string, stdout, stderr io.Writer) (int, error) {
	if len(argv) == 0 {
		return -1, nil
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(cmd.Env, env...)
	}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	// derive exit code if possible
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus(), err
		}
	}
	// could be context cancellation or other failures
	return -1, err
}
