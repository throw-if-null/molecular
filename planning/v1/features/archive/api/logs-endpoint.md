# Feature: Logs endpoint

## Goal

Implement a useful logs API for tasks/attempts.

## Current state

- Endpoint exists: `GET /v1/tasks/{task_id}/logs`
- Response is currently a placeholder JSON blob.
- Real logs exist on disk in attempt artifacts: `.molecular/runs/<task_id>/attempts/<attempt_id>/log.txt`.

## API design (v1)

Request:
- `GET /v1/tasks/{task_id}/logs`

Optional query params:
- `role=` one of `lithium|carbon|helium|chlorine` (optional)
- `attempt_id=` specific attempt id (optional)
- `tail=` number of lines from end (optional)

Response (v1):
- Content-Type: `text/plain; charset=utf-8`
- Body: log text

Error cases:
- `404` if task or attempt not found
- `400` if query params invalid

## Detailed implementation steps

1. Define selection rules
   - If `attempt_id` provided: load that attempt, return its `log.txt`.
   - Else if `role` provided: select the latest attempt for that task+role.
   - Else: select latest attempt for task across all roles.

2. Store support
   - Add `store` methods required to query attempts:
     - `GetAttempt(taskID, attemptID)`
     - `GetLatestAttempt(taskID)`
     - `GetLatestAttemptByRole(taskID, role)`
   - Ensure these are indexed in SQLite for speed (index on `attempts(task_id, role, created_at)` ideally).

3. Server handler
   - Implement handler in Silicon server routing for `/v1/tasks/{task_id}/logs`.
   - Validate query params.
   - Use store methods above to find attempt and read its artifacts dir.

4. Reading and tailing
   - Read log from `<artifacts_dir>/log.txt`.
   - If `tail` is set, implement line tailing without loading entire file for huge logs:
     - v1 simplest acceptable approach: read full file and tail in memory (cap max bytes, e.g. 1–5MB).
     - improvement: implement reverse scan for newlines.

5. Response headers
   - Set `Content-Type` to text/plain.
   - Optionally set `X-Molecular-Attempt-Id` and `X-Molecular-Role` headers.

6. Tests
   - Add server tests in `internal/silicon/server_test.go`:
     - create task+attempts in store
     - write a `log.txt` in the attempt artifacts directory
     - assert endpoint returns expected log content
   - Add tests for invalid `role` and invalid `tail`.

7. CLI integration
   - Update `molecular logs` command to:
     - call the endpoint
     - support `--tail N` by passing `?tail=N`

## Acceptance criteria

- Works for tasks with multiple attempts.
- Returns 404 for unknown task/attempt.
- Handles large logs (tailing and/or size caps).
- `molecular logs --tail` is functional.
