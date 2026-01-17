# Watch Mode

## Overview

Add real-time status monitoring with `molecular status <task-id> --watch` that polls Silicon and refreshes the display until the task reaches a terminal state.

See detailed specification: [`planning/v1/features/cli/watch-mode.md`](../../planning/v1/features/cli/watch-mode.md)

## Goals

- Continuous task monitoring without manual refresh
- Auto-exit on terminal states (completed/failed/cancelled)
- Configurable poll interval (default 2s)
- Clear visual feedback for phase transitions
- Elapsed time tracking

## Implementation Approach

1. Add `--watch` / `-w` and `--interval=N` flags to status command
2. Implement poll loop with `time.Ticker` and signal handling (Ctrl+C)
3. Clear terminal and redraw status on each poll using ANSI escape codes
4. Show elapsed time, phase history, and latest attempt status
5. Highlight phase transitions with color-coded symbols (✓ success, → running, ✗ failed)
6. Exit automatically when task reaches terminal state

## Tasks

| ID | Title | Status | Dependencies | Notes |
|----|-------|--------|--------------|-------|
| [mol-wat-001](./mol-wat-001.md) | Add watch mode flags to status command | todo | - | --watch, --interval parsing |
| [mol-wat-002](./mol-wat-002.md) | Implement poll loop with signal handling | todo | mol-wat-001 | time.Ticker + Ctrl+C |
| [mol-wat-003](./mol-wat-003.md) | Implement terminal clearing and rendering | todo | mol-wat-002 | ANSI escape codes |
| [mol-wat-004](./mol-wat-004.md) | Add phase transition highlighting | todo | mol-wat-003 | Color-coded symbols |
| [mol-wat-005](./mol-wat-005.md) | Add elapsed time and phase history display | todo | mol-wat-003 | Calculate durations |
| [mol-wat-006](./mol-wat-006.md) | Test and document watch mode | todo | mol-wat-004, mol-wat-005 | Manual testing + README |

### Task Dependency Graph

```
mol-wat-001 (flags)
    └─> mol-wat-002 (poll loop)
            └─> mol-wat-003 (terminal rendering)
                    ├─> mol-wat-004 (highlighting) ──┬─> mol-wat-006 (test + docs)
                    └─> mol-wat-005 (elapsed time) ──┘
```

## Feature Dependencies

None - extends existing `molecular status` command.

## Testing Strategy

- **Manual testing with real Silicon instance:**
  - Submit a task and watch it progress through phases
  - Verify terminal clears and redraws correctly
  - Verify elapsed time updates on each refresh
  - Verify phase transitions are highlighted
  - Test Ctrl+C exits cleanly without corrupting terminal
  - Test auto-exit on terminal state (completed/failed/cancelled)
  - Test different poll intervals (1s, 5s, 10s)
  - Test incompatible flag combination: `--watch --json` errors
- **Terminal compatibility:**
  - Test on Linux (xterm, gnome-terminal)
  - Test on macOS (Terminal.app, iTerm2)
  - Verify ANSI escape codes work correctly
- **Edge cases:**
  - Task doesn't exist (404 error, should exit)
  - Silicon not running (connection error, should exit)
  - Very fast task (completes before first poll)

## Documentation Updates

- Update usage text for `status` command in `cmd/molecular/main.go`
- Add watch mode example to README.md
- Document recommended poll intervals (2s default, adjust for network latency)
- Add note about terminal compatibility (requires ANSI support)

## Acceptance Criteria

- [ ] `molecular status <task-id> --watch` enables watch mode
- [ ] Terminal clears and redraws on each poll (no scrolling)
- [ ] Shows elapsed time since task start (e.g., "Elapsed: 1m 23s")
- [ ] Shows phase history with color-coded symbols (✓ ✗ →)
- [ ] Shows latest attempt status and duration
- [ ] Auto-exits on terminal state with clear message ("Task completed. Exiting.")
- [ ] Ctrl+C exits cleanly and restores terminal
- [ ] `--interval=N` customizes poll interval (validates N > 0)
- [ ] `--watch --json` returns error: "incompatible flags"
- [ ] Works with standard ANSI terminals (Linux, macOS)
- [ ] 404 error exits gracefully if task doesn't exist
- [ ] Connection error exits gracefully if Silicon not running
