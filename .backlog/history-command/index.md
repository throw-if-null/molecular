# History Command

## Overview

Implement `molecular history <task_id>` to show all attempts for a task with metadata, making it easy to debug task progression and understand retry/failure history.

See detailed specification: [`planning/v1/features/cli/history.md`](../../planning/v1/features/cli/history.md)

## Goals

- View all attempts for a task in chronological order
- Display attempt metadata (role, status, duration, errors)
- Support both human-readable table and JSON output
- Help users debug failed tasks and retry sequences

## Implementation Approach

1. Add store method `ListAttemptsByTask(taskID)` to query attempts table
2. Create API endpoint `GET /v1/tasks/{task_id}/attempts` returning attempt list
3. Implement CLI command `molecular history` with table rendering using `text/tabwriter`
4. Add `--json` flag for machine-readable output
5. Color-code status (green/red/yellow) and truncate error summaries in table view

## Tasks

| ID | Title | Status | Dependencies | Notes |
|----|-------|--------|--------------|-------|
| [mol-his-001](./mol-his-001.md) | Add store.ListAttemptsByTask method | todo | - | SQL query + Attempt struct |
| [mol-his-002](./mol-his-002.md) | Add GET /v1/tasks/{id}/attempts endpoint | todo | mol-his-001 | Handler in server.go |
| [mol-his-003](./mol-his-003.md) | Implement CLI history command with table output | todo | mol-his-002 | text/tabwriter, duration calc |
| [mol-his-004](./mol-his-004.md) | Add color-coding and error truncation | todo | mol-his-003 | ANSI colors for status |
| [mol-his-005](./mol-his-005.md) | Add --json flag support | todo | mol-his-003 | Marshal attempts to JSON |
| [mol-his-006](./mol-his-006.md) | Add unit tests for store/API/CLI | todo | mol-his-001, mol-his-002, mol-his-003 | Table-driven tests |
| [mol-his-007](./mol-his-007.md) | Add integration test and update docs | todo | mol-his-006 | End-to-end + README |

### Task Dependency Graph

```
mol-his-001 (store method)
    └─> mol-his-002 (API endpoint)
            └─> mol-his-003 (CLI command) ──┬─> mol-his-006 (unit tests)
                    ├─> mol-his-004 (colors)  │        └─> mol-his-007 (integration + docs)
                    └─> mol-his-005 (JSON) ───┘
```

## Feature Dependencies

None - uses existing store and API infrastructure.

## Testing Strategy

- **Unit tests:**
  - Store: Query returns attempts sorted by started_at ASC
  - Store: Empty result for nonexistent task_id
  - API: 404 for nonexistent task
  - API: Correct JSON structure
  - CLI: Table rendering with mock HTTP client
  - CLI: JSON output format
- **Integration test:**
  - Submit task, create multiple attempts (some failed), verify history output matches

## Documentation Updates

- Update `cmd/molecular/main.go` usage function to list `history` command
- Add example to README.md showing debugging workflow with history
- Update help text with `--json` flag documentation

## Acceptance Criteria

- [ ] `molecular history <task-id>` shows human-readable table with aligned columns
- [ ] Table columns: ATTEMPT_ID, ROLE, STATUS, STARTED, DURATION, ERROR
- [ ] `molecular history <task-id> --json` outputs valid JSON array
- [ ] Attempts sorted chronologically (started_at ascending)
- [ ] Error summaries truncated to ~80 chars in table view with "..." suffix
- [ ] Status color-coded: green (success), red (failed), yellow (cancelled/in_progress)
- [ ] Duration displayed in human-friendly format (2.3s, 1m 15s)
- [ ] 404 error with clear message for nonexistent task_id
- [ ] All tests passing (`go test ./internal/store/...`, `./internal/silicon/...`, `./cmd/molecular/...`)
- [ ] No performance regression (query should use index on task_id)
