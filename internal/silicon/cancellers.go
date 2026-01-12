package silicon

import (
	"context"
	"sync"
)

// Simple in-memory registry of per-task cancellers so the cancel endpoint can
// signal a running attempt quickly without waiting for a DB poll.
var (
	cancellersMu sync.Mutex
	cancellers   = map[string]context.CancelFunc{}
)

// RegisterAttemptCanceler registers a cancel func for a task id. It will
// overwrite any previous entry for the task.
func RegisterAttemptCanceler(taskID string, cancel context.CancelFunc) {
	cancellersMu.Lock()
	defer cancellersMu.Unlock()
	cancellers[taskID] = cancel
}

// UnregisterAttemptCanceler removes any registered cancel func for a task id.
func UnregisterAttemptCanceler(taskID string) {
	cancellersMu.Lock()
	defer cancellersMu.Unlock()
	delete(cancellers, taskID)
}

// CancelInMemory signals the registered cancel func for taskID if present.
// Returns true if a cancel func was found and called.
func CancelInMemory(taskID string) bool {
	cancellersMu.Lock()
	cancel, ok := cancellers[taskID]
	cancellersMu.Unlock()
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}
