# Feature: Carbon real execution

## Goal

Make Carbon actually run a builder agent and apply changes in the task worktree.

## Current state

- Carbon is a stub worker that writes attempt artifacts but does not run an agent.
- Tests simulate failures via prompt markers.

## Detailed implementation steps

1. Define the execution interface
   - Add an abstraction in `internal/silicon`:
     - `type Runner interface { Run(ctx context.Context, workdir string, args []string, env []string) (exitCode int, stdout string, stderr string, err error) }`
   - Provide a default implementation using `os/exec`.

2. Decide how to invoke the builder
   - v1 simplest: run a command configured in config, e.g.:
     - `opencode run --agent builder --task <task_id> ...`
     - or `bash -lc "..."`
   - Prefer explicit argv (avoid shell) where possible.

3. Ensure workdir correctness
   - Carbon must run inside:
     - `.molecular/worktrees/<task_id>`
   - Confirm Lithium has already created the worktree, and error out if missing.

4. Capture logs deterministically
   - Stream stdout/stderr into attempt `log.txt`.
   - Also store a short “error summary” field in DB for listing/status.

5. Normalize results
   - Define a stable `result.json` for Carbon:
     - status (`succeeded`/`failed`)
     - summary
     - exit code
     - paths to artifacts
   - Optionally include:
     - `git diff --stat`
     - changed file list

6. Update retry logic
   - Remove prompt-marker-driven failure simulation in production paths.
   - Keep prompt markers only for tests (or behind build tag), if desired.

7. Cancellation support
   - Ensure Carbon run uses a `context.Context`.
   - When task is cancelled, cancel the context and kill the process.

8. Tests
   - Add tests for:
     - runner invocation (with a fake Runner)
     - log.txt gets written
     - cancellation terminates run
     - failure retry budget behavior

9. Observability
   - Add structured logging (even just prefixes) into `log.txt`:
     - command
     - workdir
     - start/end timestamps

## Acceptance criteria

- A real task results in actual git diffs in the worktree.
- Failures are actionable from artifacts alone.
- Cancellation terminates the running process.
