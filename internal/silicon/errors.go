package silicon

import "errors"

// Common typed errors returned by silicon package operations.
var (
	ErrNotFound      = errors.New("not found")
	ErrPrereqMissing = errors.New("prerequisite missing")
	ErrHookFailed    = errors.New("hook failed")
)
