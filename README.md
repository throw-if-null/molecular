# Molecular

Molecular is a local-first workflow engine for running multi-step “agentic” coding tasks against a Git repository.

It consists of:
- **Silicon**: a long-running local daemon that owns durable state and orchestrates work.
- **molecular**: a CLI client that submits tasks and inspects state.

This repo currently focuses on a deterministic, debuggable execution model with:
- durable state in SQLite
- stable on-disk artifacts under `.molecular/`
- predictable worker phase transitions and retry semantics

## Status

This is an early implementation. Some parts are stubbed (notably Carbon/Helium “agent execution” and streaming logs), but the core APIs, persistence, artifacts layout, hooks, and retry loop scaffolding exist.

## Concepts

Molecular uses a chemistry naming theme for roles:
- **Silicon**: orchestrator (daemon)
- **Lithium**: setup worker (ensure worktree, run setup hook)
- **Carbon**: builder worker (implements changes)
- **Helium**: inspector worker (reviews changes)
- **Chlorine**: finisher worker (wrap-up hook; future PR creation)

A task progresses through phases (setup/build/review/finish) and records an **attempt** for each worker run. Attempts write artifacts to disk for debugging.

## Directory layout

Molecular stores all its state under a repo-local `.molecular/` directory.

- `.molecular/molecular.db`: SQLite database for tasks and attempts
- `.molecular/worktrees/<task_id>/`: per-task git worktree
- `.molecular/runs/<task_id>/attempts/<attempt_id>/`:
  - `meta.json`: attempt metadata
  - `result.json`: normalized result summary
  - `log.txt`: attempt log output

Optional repo hooks:
- `.molecular/lithium.sh`: executed after Lithium creates/ensures the worktree
- `.molecular/chlorine.sh`: executed during Chlorine

## Install / build

Requires Go.

Build binaries:

```sh
go build ./cmd/silicon
go build ./cmd/molecular
```

## Run Silicon

Run the daemon (defaults to `127.0.0.1:8711`):

```sh
./silicon
```

Health check:

```sh
curl -s http://127.0.0.1:8711/healthz
```

## Use the CLI

Submit a task (idempotent by `--task-id`):

```sh
./molecular submit --task-id demo --prompt "Do something useful"
```

Check status:

```sh
./molecular status demo
```

List tasks:

```sh
./molecular list --limit 20
```

Cancel a task:

```sh
./molecular cancel demo
```

Fetch logs (currently a placeholder response; artifacts contain attempt logs):

```sh
./molecular logs demo
```

## Doctor

`molecular doctor` checks local prerequisites and repo wiring.

```sh
./molecular doctor
./molecular doctor --json
```

Checks include:
- `git` available in `PATH` (required)
- `gh` (GitHub CLI) in `PATH` (optional)
- `.molecular/config.toml` exists
- `.molecular/lithium.sh` and `.molecular/chlorine.sh` exist and are executable

Exit codes:
- `0`: ok
- `1`: problems found
- `2`: usage error

## HTTP API (Silicon)

Primary endpoints:
- `GET /healthz`
- `POST /v1/tasks` (create/get by `task_id`)
- `GET /v1/tasks/{task_id}`
- `GET /v1/tasks` (supports `?limit=`)
- `POST /v1/tasks/{task_id}/cancel`
- `GET /v1/tasks/{task_id}/logs` (currently a placeholder JSON response)

## Roadmap

High-level roadmap lives in `ROADMAP.md`.

For a more structured “v1 product” checklist, see `planning/v1/TODO.md`.
