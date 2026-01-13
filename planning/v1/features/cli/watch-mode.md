# Feature: `--watch` Mode for `molecular status`

## Goal

Add real-time status monitoring with `molecular status <task-id> --watch` that polls Silicon and refreshes the display until the task reaches a terminal state.

## Current state

- `molecular status` shows static snapshot
- Users must manually run `status` repeatedly to monitor progress
- No way to continuously monitor task without scripting

## Requirements

### Watch mode behavior

```bash
molecular status <task-id> --watch [--interval=N]
```

**Behavior:**
- Poll Silicon every N seconds (default: 2s)
- Clear terminal and redraw status on each poll
- Show elapsed time since task started
- Highlight phase transitions (lithium → build → review → finish)
- Exit automatically when task reaches terminal state (completed/failed/cancelled)
- Support Ctrl+C to exit early

### Flags

- `--watch` / `-w`: Enable watch mode
- `--interval=N`: Poll interval in seconds (default: 2)
- `--json`: Incompatible with `--watch` (error if both specified)

### Terminal rendering

**Display format:**
```
Watching task: feat-123 [Elapsed: 45s] [Refreshes every 2s]
Press Ctrl+C to exit

Task: feat-123
Status: running
Phase: build
Created: 2026-01-13 10:00:00
Started: 2026-01-13 10:00:02
Updated: 2026-01-13 10:00:45

Latest attempt:
  ID: carbon-def456
  Role: carbon
  Status: running
  Started: 2026-01-13 10:00:15
  Duration: 30s

Phase history:
  ✓ lithium (2.3s) → success
  → build (30s+) → running

Last refreshed: 2026-01-13 10:00:45
```

**Terminal state reached:**
```
Task: feat-123
Status: completed
Phase: finish
...

Task completed. Exiting.
```

## Detailed implementation steps

### 1. Add watch flags to CLI

**File:** `cmd/molecular/main.go`

**Changes:**
- Add `--watch` / `-w` flag parsing for `status` command
- Add `--interval=N` flag parsing (default: 2)
- Validate: error if `--json` and `--watch` both specified

### 2. Implement watch loop

**File:** `cmd/molecular/main.go` (status command)

**Implementation:**
```go
func handleStatus(args []string) {
    taskID := args[0]
    watch := hasFlagBool(args, "--watch", "-w")
    interval := getFlagInt(args, "--interval", 2)
    jsonOutput := hasFlagBool(args, "--json")
    
    if watch && jsonOutput {
        fmt.Fprintln(os.Stderr, "error: --watch and --json are incompatible")
        os.Exit(2)
    }
    
    if !watch {
        // Existing single-shot behavior
        printStatus(taskID, jsonOutput)
        return
    }
    
    // Watch mode
    startTime := time.Now()
    ticker := time.NewTicker(time.Duration(interval) * time.Second)
    defer ticker.Stop()
    
    // Set up signal handling for Ctrl+C
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    
    for {
        select {
        case <-sigCh:
            fmt.Println("\nExiting...")
            return
        case <-ticker.C:
            task, err := fetchTaskStatus(taskID)
            if err != nil {
                fmt.Fprintf(os.Stderr, "error fetching status: %v\n", err)
                return
            }
            
            clearScreen()
            printWatchHeader(taskID, startTime, interval)
            printStatus(taskID, false)
            printWatchFooter()
            
            // Exit if terminal state reached
            if isTerminalStatus(task.Status) {
                fmt.Printf("\nTask %s. Exiting.\n", task.Status)
                return
            }
        }
    }
}

func clearScreen() {
    // ANSI escape sequence to clear screen and move cursor to top-left
    fmt.Print("\033[2J\033[H")
}

func printWatchHeader(taskID string, startTime time.Time, interval int) {
    elapsed := time.Since(startTime)
    fmt.Printf("Watching task: %s [Elapsed: %s] [Refreshes every %ds]\n", taskID, fmtDuration(elapsed), interval)
    fmt.Println("Press Ctrl+C to exit\n")
}

func printWatchFooter() {
    fmt.Printf("\nLast refreshed: %s\n", time.Now().Format("2006-01-02 15:04:05"))
}

func isTerminalStatus(status string) bool {
    return status == "completed" || status == "failed" || status == "cancelled"
}
```

### 3. Enhanced status display for watch mode

**File:** `cmd/molecular/main.go`

**Changes:**
- Add phase history timeline
- Show elapsed time for current phase
- Highlight transitions (use ANSI colors if terminal supports)

**Example with colors:**
```go
func printPhaseHistory(task Task) {
    fmt.Println("\nPhase history:")
    
    // This would require tracking phase transitions in the store
    // For v1, we can show current phase + latest attempt
    
    if task.Phase == "lithium" {
        fmt.Println("  → lithium → running")
    } else if task.Phase == "build" {
        fmt.Println("  ✓ lithium → success")
        fmt.Println("  → build → running")
    }
    // ... etc for other phases
}
```

### 4. Tests

**Manual testing:**
- Start Silicon and submit a task
- Run `molecular status <task-id> --watch`
- Verify display refreshes every 2s
- Verify Ctrl+C exits cleanly
- Verify exits when task completes

**Unit tests:**
- `isTerminalStatus` returns true for completed/failed/cancelled
- `isTerminalStatus` returns false for running/pending
- Flag parsing: `--watch` and `--json` error when combined
- Interval parsing: defaults to 2s

**Edge cases:**
- Watch mode when task already in terminal state (should print once and exit)
- Watch mode when Silicon is down (should error gracefully)

### 5. Update help text

**File:** `cmd/molecular/main.go`

**Update usage:**
```
molecular status <task-id> [--json] [--watch] [--interval N]
```

**Add to help text:**
```
--watch, -w       Watch mode: continuously refresh status until task completes
--interval N      Poll interval in seconds for watch mode (default: 2)
```

### 6. Update documentation

**File:** `README.md`

**Add example:**
```markdown
## Real-time monitoring

Watch a task in real-time:

\```bash
molecular status feat-123 --watch
\```

Customize refresh interval:

\```bash
molecular status feat-123 --watch --interval=5
\```
```

## Acceptance criteria

- [ ] `--watch` flag enables continuous monitoring
- [ ] `--interval` customizes poll frequency
- [ ] Display clears and refreshes on each poll
- [ ] Elapsed time shown in header
- [ ] Ctrl+C exits cleanly
- [ ] Auto-exits on terminal state
- [ ] Error when `--watch` and `--json` combined
- [ ] Help text updated
- [ ] Documentation updated

## Example usage

### Scenario: Monitor a long-running task

```bash
# Submit task
$ molecular submit --task-id feat-123 --prompt "implement feature X"
Task created: feat-123

# Start watching
$ molecular status feat-123 --watch
Watching task: feat-123 [Elapsed: 0s] [Refreshes every 2s]
Press Ctrl+C to exit

Task: feat-123
Status: running
Phase: lithium
...

[2 seconds later, display refreshes]

Watching task: feat-123 [Elapsed: 2s] [Refreshes every 2s]
Press Ctrl+C to exit

Task: feat-123
Status: running
Phase: build
...

[continues refreshing until task completes]

Task: feat-123
Status: completed
Phase: finish
...

Task completed. Exiting.
```

## Follow-up work (post-v1)

- Colorized output (green for success, red for failed, yellow for running)
- Show live log tail in watch mode
- Show progress bar for known-duration operations
- Support multiple task watches simultaneously (split-screen)
- Export watch session to log file
