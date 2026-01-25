# T001 - Implement Silicon Base (skeleton + TDD)

| field | value |
| --- | --- |
| status | TODO |
| started_on |  |
| finished_on |  |

## Goal

Re-introduce a **minimal Silicon daemon** that supports a **single running task** (no queue), driven by **tests first** (red → green).

This is the first step of rebuilding Silicon after purging the old worker/polling + SQLite architecture.

## Non-goals (explicitly out of scope for T001)

- No SQLite / durable DB
- No polling workers
- No Carbon/Helium/Lithium/Chlorine execution logic (those will come later)
- No worktrees or on-disk artifacts
- No logs streaming / artifacts endpoints (can be stubs)

## Design decisions (v1)

- **Single task at a time**: if any task is `running`, new `POST /v1/tasks` fails fast with `409 Conflict`.
- `POST /v1/tasks` **returns immediately** (task is executed asynchronously).
- In-memory state only; Silicon restart loses state (acceptable for v1).
- Execution is injected behind an interface so we can test Silicon without implementing Carbon/Helium.
- No new dependencies.

## HTTP API contract (initial)

Keep endpoints aligned with the current CLI if convenient, but adjust as needed.

### `GET /healthz`

- **200 OK**
- Body: `ok\n` (or JSON; choose one and test it)

### `POST /v1/tasks`

Request JSON:

```json
{"task_id":"demo","prompt":"do thing"}
```

Responses:

- **201 Created** (or 200 OK; pick one and test it)
  - Body: JSON task representation
- **409 Conflict** if another task is currently running
- **400 Bad Request** on invalid JSON / missing fields

### `GET /v1/tasks/{task_id}`

- **200 OK** with JSON task
- **404 Not Found** if unknown

### (Optional for T001-2) `GET /v1/tasks`

- **200 OK** with JSON array of tasks (may be only current + recent in-memory)

### (Optional for T001-2) `POST /v1/tasks/{task_id}/cancel`

- **200 OK** with small JSON body (e.g. `{"cancelled":true}`)
- If task not found: **404**
- If task already terminal: can be **409** or **200** (idempotent) — decide and test.

## Task model (suggested)

Minimum required fields:

- `task_id` (string)
- `prompt` (string)
- `status` (string): `running|completed|failed|cancelled`
- `phase` (string): for now can be `run` or empty (we can keep `phase` to avoid rewriting CLI later)
- `created_at`, `updated_at` (RFC3339)
- optional `error` field for failures

You can reuse `internal/api` structs if that reduces churn, but it’s fine to define a Silicon-local response shape if we also update the CLI in a later task.

## Internal contracts for implementation (for @gopher)

### Packages/files to (re)introduce

- `cmd/silicon/main.go` – start HTTP server (default host/port OK)
- `internal/silicon/server.go` – HTTP handler wiring
- `internal/silicon/server_test.go` – TDD tests using `httptest`

### Execution interface (must be injectable)

One of the following is fine; pick the simplest and keep it stable:

Option A:

```go
type TaskExecutor interface {
    Execute(ctx context.Context, taskID string, prompt string) error
}
```

Option B:

```go
type TaskExecutor interface {
    Execute(ctx context.Context, t Task) error
}
```

Server constructor should accept this executor so tests can pass a fake.

### Server surface (suggested)

```go
package silicon

type Server struct {
    // ...
}

func NewServer(exec TaskExecutor) *Server
func (s *Server) Handler() http.Handler
```

## Subtasks (each is its own feature branch)

### T001-1: Silicon skeleton + TDD for healthz + submit/status

Branch: `feature/T001-1-silicon-skeleton`

Red tests (must exist before implementation):

1. `TestHealthz`
2. `TestSubmitStartsTaskAndEventuallyCompletes`

Implementation notes:

- Task starts asynchronously on submit
- Use a fake executor in tests that can be controlled (channels) to simulate long-running work

Definition of done:

- `go test ./...` passes
- No new deps
- Minimal API working

@gopher report back to @gommander with:

- what changed
- tests run
- any open questions / follow-ups

### T001-2: Fail-fast single-task constraint + cancel (+ list optional)

Branch: `feature/T001-2-single-task-and-cancel`

Red tests:

1. `TestSecondSubmitWhileRunningReturns409`
2. `TestCancelStopsRunningTask` (if cancel endpoint included)

Definition of done:

- `POST /v1/tasks` returns 409 while a task is running
- cancellation propagates via context cancellation to the executor
- `go test ./...` passes

@gopher report back to @gommander with the same format.

### T001-3 (optional): Stub endpoints to reduce CLI breakage

Branch: `feature/T001-3-stub-logs-cleanup`

- `GET /v1/tasks/{id}/logs` returns `501 Not Implemented` (or a small placeholder)
- `POST /v1/tasks/{id}/cleanup` returns `501 Not Implemented`

Only do this if it meaningfully helps manual use.

## Notes for PR (when T001 fully complete)

Once T001-1 and T001-2 are implemented and reviewed by @gommander, @gommander will open a PR for review.
