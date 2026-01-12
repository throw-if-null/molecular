# Feature: Crash recovery

## Goal

Define and implement Silicon crash/restart behavior.

## Current state

- There is limited “in-flight attempt reconciliation” behavior.

## Detailed implementation steps

1. Define invariants
   - At most one active attempt per task.
   - Attempts are append-only history.

2. On startup reconciliation routine
   - Query for attempts/tasks in “running” state.
   - Mark them as failed due to restart, with error summary.
   - If budget allows, re-enqueue task for the appropriate phase.

3. Ensure idempotency
   - Reconciliation should be safe to run multiple times.

4. Tests
   - Create a task with an attempt in running state.
   - Restart simulation: create a new Silicon instance.
   - Confirm state is updated as expected.

## Acceptance criteria

- Restarting Silicon does not leave tasks stuck.
- Failures are visible and attributable to restart.
