package paths

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	// ErrInvalidTaskID returned when task id fails validation
	ErrInvalidTaskID = errors.New("invalid task id")
)

const maxTaskIDLen = 64

// MaxTaskIDLen returns the maximum allowed task id length.
func MaxTaskIDLen() int { return maxTaskIDLen }

var taskIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,` + strconv.Itoa(maxTaskIDLen) + `}$`)

// ValidateTaskID returns nil for allowed task ids, or ErrInvalidTaskID.
// Rules:
// - Only allow ASCII letters, digits, dot, underscore and dash.
// - Max length is 64.
// - Disallow any ".." substring to avoid traversal attempts.
// - This forbids path separators '/' and '\\' and characters like ':' used in drive letters.
func ValidateTaskID(id string) error {
	if id == "" {
		return fmt.Errorf("empty task id: %w", ErrInvalidTaskID)
	}
	if len(id) > maxTaskIDLen {
		return fmt.Errorf("task id too long: %w", ErrInvalidTaskID)
	}
	if strings.Contains(id, "..") {
		return fmt.Errorf("task id contains disallowed '..': %w", ErrInvalidTaskID)
	}
	if !taskIDRe.MatchString(id) {
		return fmt.Errorf("task id contains invalid characters: %w", ErrInvalidTaskID)
	}
	return nil
}

// RunsDir returns the relative runs directory for a task (e.g. ".molecular/runs/<task>").
func RunsDir(taskID string) (string, error) {
	if err := ValidateTaskID(taskID); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(".molecular", "runs", taskID)), nil
}

// WorktreeDir returns the relative worktree path for a task (e.g. ".molecular/worktrees/<task>").
func WorktreeDir(taskID string) (string, error) {
	if err := ValidateTaskID(taskID); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(".molecular", "worktrees", taskID)), nil
}

// AttemptDir returns the relative attempt artifacts dir for given task and attempt id.
func AttemptDir(taskID string, attemptID int64) (string, error) {
	if err := ValidateTaskID(taskID); err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(".molecular", "runs", taskID, "attempts", fmt.Sprintf("%d", attemptID))), nil
}

// SafeJoin joins repoRoot with rel and ensures the resulting path is inside repoRoot.
// Returns an error if the result would escape repoRoot or if inputs are absolute in unexpected ways.
func SafeJoin(repoRoot, rel string) (string, error) {
	if repoRoot == "" {
		return "", fmt.Errorf("empty repo root")
	}
	// If rel is absolute, joining will return rel; treat absolute rel as disallowed.
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("relative path expected, got absolute: %s", rel)
	}
	joined := filepath.Join(repoRoot, rel)
	cleaned := filepath.Clean(joined)
	// Make both absolute for reliable Rel behavior
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	absCleaned, err := filepath.Abs(cleaned)
	if err != nil {
		return "", err
	}
	relToRoot, err := filepath.Rel(absRoot, absCleaned)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(relToRoot, "..") || strings.HasPrefix(filepath.ToSlash(relToRoot), "../") {
		return "", fmt.Errorf("path escapes repo root: %s", rel)
	}
	return absCleaned, nil
}
