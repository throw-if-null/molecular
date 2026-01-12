package paths_test

import (
	"strings"
	"testing"

	"github.com/throw-if-null/molecular/internal/paths"
)

func TestValidateTaskIDGood(t *testing.T) {
	good := []string{"task-1", "a", "A0._-"}
	for _, s := range good {
		if err := paths.ValidateTaskID(s); err != nil {
			t.Fatalf("expected valid for %q, got %v", s, err)
		}
	}
}

func TestValidateTaskIDBad(t *testing.T) {
	bad := []string{``, `a/b`, `a\b`, `../x`, `..\x`, `/abs`, `C:\x`, `a b`}
	for _, s := range bad {
		if err := paths.ValidateTaskID(s); err == nil {
			t.Fatalf("expected invalid for %q", s)
		}
	}
	long := strings.Repeat("x", paths.MaxTaskIDLen()+1)
	if err := paths.ValidateTaskID(long); err == nil {
		t.Fatalf("expected invalid for long task id of length %d", len(long))
	}
}
