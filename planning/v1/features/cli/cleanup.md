# Feature: Cleanup semantics

## Goal

Define and implement what `molecular cleanup <task_id>` does.

## Current state

- CLI command exists, but behavior is placeholder.

## Detailed implementation steps

1. Define scope
   - Decide which resources cleanup removes:
     - `.molecular/worktrees/<task_id>`
     - `.molecular/runs/<task_id>`
     - task rows in SQLite

2. Add a Silicon endpoint
   - Consider implementing server-side cleanup to keep policy centralized:
     - `POST /v1/tasks/{task_id}/cleanup`

3. Implement safe deletion
   - Validate `task_id` formatting.
   - Resolve absolute paths and ensure they are under `.molecular/`.
   - Refuse to delete anything outside `.molecular/`.

4. DB policy
   - Option A (recommended): keep DB history, only delete worktree + artifacts.
   - Option B: delete task and related attempts.

5. Tests
   - Ensure cleanup is idempotent.
   - Ensure cleanup refuses path traversal task ids.
   - Ensure cleanup deletes the expected directories only.

## Acceptance criteria

- Cleanup is safe and predictable.
- Cleanup is idempotent.
- Behavior is documented in README.
