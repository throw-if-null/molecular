# Feature: `molecular history` Command

## Goal

Implement `molecular history <task_id>` to show all attempts for a task with metadata, making it easy to debug task progression and understand retry/failure history.

## Current state

- Attempts are stored in SQLite (`attempts` table)
- No CLI command to view attempt history
- Users must query SQLite directly or examine artifacts on disk

## Requirements

### CLI interface

```bash
molecular history <task-id>           # Human-readable table
molecular history <task-id> --json    # JSON output for scripting
```

### Output format (human-readable)

```
Task: feat-123
Status: completed
Phase: finish

Attempts (5 total):

ATTEMPT ID        ROLE      STATUS     STARTED              DURATION  ERROR
lithium-abc123    lithium   success    2026-01-13 10:00:00  2.3s      -
carbon-def456     carbon    failed     2026-01-13 10:00:03  45.2s     build failed
carbon-ghi789     carbon    success    2026-01-13 10:01:15  38.1s     -
helium-jkl012     helium    success    2026-01-13 10:02:00  12.5s     -
chlorine-mno345   chlorine  success    2026-01-13 10:02:15  5.2s      -

Use 'molecular logs <task-id> --attempt-id=<id>' to view attempt logs.
```

### Output format (JSON)

```json
{
  "task_id": "feat-123",
  "status": "completed",
  "phase": "finish",
  "attempts": [
    {
      "attempt_id": "lithium-abc123",
      "role": "lithium",
      "status": "success",
      "started_at": "2026-01-13T10:00:00Z",
      "finished_at": "2026-01-13T10:00:02Z",
      "duration_ms": 2300,
      "error_summary": null,
      "artifacts_dir": ".molecular/runs/feat-123/attempts/lithium-abc123"
    },
    ...
  ]
}
```

### Error highlighting

- Failed attempts should be visually distinct (red text in terminal)
- Error summary should be truncated to ~80 chars in table view
- Full error available via `--json` or logs endpoint

## Detailed implementation steps

### 1. Add Silicon API endpoint

**Endpoint:** `GET /v1/tasks/{task_id}/attempts`

**Response:**
```json
{
  "task_id": "...",
  "attempts": [
    {
      "attempt_id": "...",
      "role": "...",
      "status": "...",
      "started_at": "...",
      "finished_at": "...",
      "error_summary": "..."
    }
  ]
}
```

**Handler location:** `internal/silicon/server.go`

**Implementation:**
- Parse `task_id` from URL path
- Call store method `ListAttemptsByTask(taskID)`
- Validate task exists (404 if not)
- Return attempts sorted by `started_at` ascending
- Include error summary for failed attempts

### 2. Add store method

**Method:** `ListAttemptsByTask(taskID string) ([]Attempt, error)`

**File:** `internal/store/store.go`

**Implementation:**
```sql
SELECT attempt_id, task_id, role, status, started_at, finished_at, error_summary, artifacts_dir
FROM attempts
WHERE task_id = ?
ORDER BY started_at ASC
```

**Notes:**
- Reuse existing `Attempt` struct
- Calculate duration in Go if needed (finished_at - started_at)

### 3. Add CLI command

**File:** `cmd/molecular/main.go`

**Command structure:**
```go
func handleHistory(args []string) {
    // Parse args
    taskID := args[0]
    jsonOutput := hasFlagBool(args, "--json")
    
    // Call API
    resp := httpClient.Get("/v1/tasks/" + taskID + "/attempts")
    
    // Render output
    if jsonOutput {
        printJSON(resp)
    } else {
        printHistoryTable(resp)
    }
}
```

**Table rendering:**
- Use `text/tabwriter` for aligned columns
- Color-code status: green (success), red (failed), yellow (cancelled)
- Truncate error summaries with "..." suffix
- Show human-friendly durations (2.3s, 1m 15s, etc.)

### 4. Update help text

**File:** `cmd/molecular/main.go`

**Add to usage:**
```
molecular history <task-id> [--json]
```

**Add to help text:**
```
history shows all attempts for a task with metadata (role, status, duration, errors)
```

### 5. Tests

**Unit tests:**
- `ListAttemptsByTask` returns correct attempts
- `ListAttemptsByTask` returns empty list for nonexistent task
- `ListAttemptsByTask` sorts by started_at ascending

**Integration tests:**
- Submit task, create multiple attempts, verify history output
- Test `--json` flag
- Test with failed attempts (error summary displayed)

**CLI tests:**
- `molecular history <task-id>` produces table output
- `molecular history <task-id> --json` produces valid JSON
- Nonexistent task returns error

## Acceptance criteria

- [ ] `molecular history <task-id>` shows human-readable table
- [ ] `molecular history <task-id> --json` outputs valid JSON
- [ ] Error summaries highlighted for failed attempts
- [ ] Duration calculated and displayed
- [ ] Attempts sorted chronologically
- [ ] 404 for nonexistent tasks
- [ ] Help text updated
- [ ] All tests passing

## Example usage

### Scenario: Debugging a failed task

```bash
# Check task status
$ molecular status feat-123
Task: feat-123
Status: failed
Phase: build

# View attempt history
$ molecular history feat-123
Task: feat-123
Status: failed
Phase: build

Attempts (3 total):

ATTEMPT ID        ROLE      STATUS     STARTED              DURATION  ERROR
lithium-abc123    lithium   success    2026-01-13 10:00:00  2.3s      -
carbon-def456     carbon    failed     2026-01-13 10:00:03  45.2s     pnpm test failed: 2 tests failed
carbon-ghi789     carbon    failed     2026-01-13 10:01:15  38.1s     pnpm test failed: 1 test failed

# View logs for specific attempt
$ molecular logs feat-123 --attempt-id=carbon-ghi789
[attempt log output...]
```

## Follow-up work (post-v1)

- Filter attempts by role: `molecular history <task-id> --role=carbon`
- Filter by status: `molecular history <task-id> --status=failed`
- Show retry counts in output
- Include artifacts size in output
