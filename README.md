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

This is an early implementation with a working end-to-end pipeline. All four workers (Lithium, Carbon, Helium, Chlorine) are implemented and execute real commands with streaming logs, retry semantics, and cancellation support.

## Concepts

Molecular uses a chemistry naming theme for roles:
- **Silicon**: orchestrator (daemon)
- **Lithium**: setup worker (ensure worktree, run setup hook)
- **Carbon**: builder worker (implements changes)
- **Helium**: inspector worker (reviews changes)
- **Chlorine**: finisher worker (commits changes and creates PR)

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

Run the daemon (defaults to `127.0.0.1:8711`). You can use either the built binary or `go run`.

Built binary:

```sh
./silicon
```

(or)

```sh
go run ./cmd/silicon
```

Health check:

```sh
curl -s http://127.0.0.1:8711/healthz
```

## End-to-end demo


This repo includes a working end-to-end pipeline:
- Lithium creates/ensures the worktree (and optionally runs a hook)
- Carbon runs the configured builder command and makes changes
- Helium runs the configured inspector command, reviews changes, and decides (approved/changes_requested/rejected)
- Chlorine commits changes to a task branch and creates a PR


### 0) (Optional) configure hooks

If you want to see hook execution in attempt logs:

```sh
mkdir -p .molecular
cat > .molecular/lithium.sh <<'EOF'
#!/bin/sh
set -eu

echo "lithium hook ran"
EOF
chmod +x .molecular/lithium.sh

cat > .molecular/chlorine.sh <<'EOF'
#!/bin/sh
set -eu

echo "chlorine hook ran"
EOF
chmod +x .molecular/chlorine.sh
```

### 1) Build

```sh
go build ./cmd/silicon

go build ./cmd/molecular
```

### 2) Start Silicon

In one terminal:

```sh
./silicon
```

### 3) Submit a task

In another terminal:

```sh
./molecular submit --task-id demo --prompt "hello world"
```

### 4) Watch status until completion

`molecular status` will show task phase/status and the latest attempt summary.

```sh
./molecular status demo
```

For scripting/debugging:

```sh
./molecular status --json demo
```

### 5) Inspect logs and artifacts

Fetch attempt logs via the logs endpoint (note: logs have a hard size cap):

```sh
./molecular logs demo
./molecular logs demo --tail 200
```

You can also inspect artifacts directly on disk:

```sh
ls -R .molecular/runs/demo
```

### 6) Try cancellation

Cancellation sets task status to `cancelled` and attempts try to stop quickly.

```sh
./molecular cancel demo
```

### 7) Cleanup worktree + artifacts

Cleanup is idempotent and only deletes repo-local filesystem state:
- `.molecular/worktrees/<task_id>`
- `.molecular/runs/<task_id>`

DB history is preserved.

```sh
./molecular cleanup demo
```

### Notes on future “real execution”

Carbon and Helium are intended to run external commands (e.g. `opencode run --agent carbon ...` / `opencode run --agent helium ...`) and stream stdout/stderr into attempt logs. Cancellation is designed to terminate those processes via context cancellation.


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

Fetch logs:

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
- `GET /v1/tasks/{task_id}/logs` (text/plain; supports `?tail=` and filters)

## Roadmap

High-level roadmap lives in `ROADMAP.md`.

For a more structured “v1 product” checklist, see `planning/v1/README.md`.
