# Feature: Safety and path validation

## Goal

Prevent footguns and obvious security mistakes in a local-first tool.

## Detailed implementation steps

1. Define allowed `task_id` format
   - e.g. `[a-zA-Z0-9._-]+` with a max length.
   - Reject any `task_id` with path separators.

2. Centralize path building
   - Add helpers for:
     - runs dir
     - worktree dir
     - attempt dir
   - These helpers should validate inputs and return errors.

3. Apply path validation everywhere
   - Task creation
   - Worktree creation
   - Logs endpoint
   - Cleanup

4. Tests
   - Unit tests for validation.
   - Tests for traversal attempts like `../x`, `..\\x`.

## Acceptance criteria

- Attempting traversal via `task_id` is rejected.
- Cleanup/logs cannot access outside `.molecular/`.
