# Molecular roadmap

Molecular is a local-first workflow engine for running multi-step “agentic” coding tasks against any Git repository.

Chemistry naming theme (canonical):
- Silicon: orchestrator (daemon)
- Lithium: setup worker
- Carbon: builder worker
- Helium: inspector worker
- Chlorine: finisher worker
- Phosphorus: reporter worker (future)

## Goals
- Generic by default (no repo-specific assumptions)
- Local-first and cross-platform
- Durable state with SQLite
- Predictable, debuggable retry loops
- Clear artifacts under `.molecular/`

## Non-goals (v1)
- Distributed execution / multi-host scheduling
- High-concurrency throughput
- Web UI or live log streaming
- Reusing the legacy Python runner scripts

## v1 (MVP)

### Silicon daemon
- Run Silicon as a long-lived local service.
- Bind to `127.0.0.1:8711` by default.
- Own all durable state in `.molecular/molecular.db`.
- Own all artifacts in `.molecular/runs/<task_id>/...`.
- Own worktrees in `.molecular/worktrees/<task_id>/...`.

### molecular CLI
- `molecular submit --task-id ... --prompt ...`
  - Retry-safe UX: if `task_id` exists, return existing task state.
- `molecular status <task_id>`
- `molecular list`
- `molecular cancel <task_id>`
- `molecular logs <task_id> [--tail N]`
- `molecular cleanup <task_id>`

### Workers (in-process pools)
- Implement worker pools as goroutines managed by Silicon.
- One worker instance may process exactly one task at a time.
- Roles:
  - Lithium: create/ensure worktree; run optional repo hooks.
  - Carbon: run builder agent; write normalized result artifacts.
  - Helium: run inspector agent; write normalized result artifacts.
  - Chlorine: create PR using `gh` CLI and write final summary.

### Configuration (defaults + overrides)
- Defaults should work without config.
- Allow overrides via a per-repo directory, `.molecular/`:
  - suggested config file: `.molecular/config.toml` (format TBD)
  - optional scripts:
    - `.molecular/lithium.sh` (setup hook)
    - `.molecular/chlorine.sh` (finisher hook)
- Lithium does nothing by default except ensure the worktree.

### Persistence + idempotency
- SQLite tables:
  - `tasks`: task state, retry budgets/counters, prompt, paths
  - `attempts`: per-step attempt history + artifacts directory
- Silicon is the only writer of task state.
- Crash model:
  - if Silicon restarts, any in-flight attempt is marked failed and rerun as a new attempt (new session).

### Retry semantics (matching the legacy Foreman logic)
- Two kinds of retries:
  - transient retries per role (Carbon/Helium), with independent budgets
  - full review retries when Helium requests changes, with its own budget

## v1.1
- Add `molecular doctor` to verify prerequisites:
  - `git`, and optionally `gh`, plus configured hooks.
- Improve error typing and failure summaries.
- Add a compact JSON summary output for scripting.

## v2
- Phosphorus reporter worker
  - generate an HTML report and bundle artifacts
  - optionally publish report to a local file path

## Nice-to-haves
- Optional template system for prompts with `embed` defaults.
- Optional config to run checks (lint/test) pre-PR.
- Support multiple repo roots (if Silicon manages tasks across repos).
