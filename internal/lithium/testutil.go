package lithium

import (
	"bytes"
	"context"
	"os/exec"
)

func runCmd(dir string, name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	return cmd.Run()
}
