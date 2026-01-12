# v1 product checklist

This file tracks the work needed to ship a real v1 of Molecular (beyond the current “scaffolding”).

Guiding principles:
- Keep the default experience working with **zero config**.
- Prioritize **debuggability**: every failure should be explainable from artifacts.
- Prefer a few stable primitives over lots of options.

## P0 (must-have)

### Configuration
- Parse `.molecular/config.toml` (or change format) and define a stable schema.
- Support config discovery:
  - repo-local `.molecular/config.*`
  - optional CLI overrides / env overrides
- Define worker settings (budgets, timeouts, concurrency) in config.

### Carbon: real builder execution
- Execute a builder agent (initially via `opencode` or a simple command runner).
- Ensure the builder runs **inside the per-task worktree**.
- Capture stdout/stderr into attempt `log.txt`.
- Normalize results into `result.json`:
  - success / error
  - summary
  - changed files (optional but useful)
- Decide how Carbon provides “patches” / code changes: direct edits in worktree is fine.

### Helium: real inspector execution
- Run inspector agent after Carbon.
- Define a stable review result contract:
  - `approved`
  - `changes_requested` (+ actionable items)
  - `rejected` (fatal)
- Ensure review-loop retries are driven by structured output, not prompt markers.

### Logs endpoint
- Implement `GET /v1/tasks/{task_id}/logs` to return attempt logs.
  - Start with returning the latest attempt log.
  - Add `?role=` and `?attempt_id=` filters.
  - Add `?tail=` behavior.

### Cancellation semantics
- Define cancellation behavior per phase:
  - stop picking up new work
  - if a worker is actively running an external process, terminate it
  - mark attempts/tasks with a clear cancelled status

### Cleanup semantics
- Define `molecular cleanup <task_id>` behavior:
  - which directories are removed?
  - whether DB rows are deleted or retained
  - guardrails to avoid deleting wrong paths

### Security / safety
- Ensure all on-disk paths are validated (no path traversal via task IDs).
- Ensure hooks are opt-in and documented.
- Consider “deny network” mode for agents (future).

## P1 (should-have)

### Chlorine: create PR
- Use `gh` to create PRs from worktree branches.
- Include attempt summary into PR body.
- Handle `gh` not installed gracefully.

### Better status UX
- Expand `molecular status` output with phase + attempt summary.
- Add `molecular logs --tail` real behavior.

### Crash recovery
- On Silicon restart, ensure in-flight attempt handling is correct and visible.

## P2 (nice-to-have)

### Reporting
- Add a “reporter” worker (Phosphorus) to bundle artifacts.

### Multi-repo support
- Allow Silicon to manage multiple repo roots.

## Workflow (feature branches)

Using feature branches is a good idea here.

Suggested workflow:
- Create branch from `main`: `git switch -c feature/<name>`
- Implement feature + tests; keep commits focused.
- Open a PR and merge back to `main` when green.

Suggested branch names:
- `feature/config`
- `feature/logs-endpoint`
- `feature/carbon-exec`
- `feature/helium-exec`
- `feature/pr-creation`
