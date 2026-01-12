package lithium

import (
	"bytes"
	"context"
	"os/exec"
)

// RealExecRunner runs actual commands.
type RealExecRunner struct{}

func (r *RealExecRunner) Run(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := cmd.Run()
	return b.String(), err
}
