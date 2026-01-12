# Feature: Cancellation semantics

## Goal

Make cancellation reliable and consistent across phases.

## Current state

- Cancel endpoint exists, but cancellation does not terminate running processes (since workers are mostly stubbed).

## Detailed implementation steps

1. Define task/attempt state model
   - Define what it means for a task to be cancelled:
     - terminal state vs a “stop requested” flag

2. Propagate cancellation to workers
   - Worker loops should check cancellation before starting work.
   - If a worker is running an external command, it must use a cancellable context.

3. Add per-task context tracking
   - In Silicon, track active runs by `task_id` so cancel can signal the worker.

4. DB + artifacts
   - Record cancellation in task row.
   - Attempt `result.json` should clearly state it was cancelled.

5. Tests
   - Start a fake long-running runner.
   - Call cancel endpoint.
   - Assert the runner was stopped and status updated.

## Acceptance criteria

- `molecular cancel <task_id>` stops progress quickly.
- Status reflects cancelled state clearly.
