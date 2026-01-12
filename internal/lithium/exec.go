package lithium

import "context"

// ExecRunner abstracts execution of external commands for testability.
type ExecRunner interface {
	// Run executes command with args in given dir. It should return stdout+stderr and error.
	Run(ctx context.Context, dir string, name string, args ...string) (string, error)
}
