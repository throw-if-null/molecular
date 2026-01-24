# Molecular

This repository currently contains the **`molecular` CLI**.

The Silicon daemon/orchestrator and the older worker-based execution model have been purged and are being rebuilt.

## Build

```sh
go build ./cmd/molecular
```

## CLI usage

```sh
molecular submit --task-id <id> --prompt <text>
molecular status <task-id>
molecular list [--limit N]
molecular cancel <task-id>
molecular logs <task-id> [--tail N]
molecular cleanup <task-id>
molecular doctor [--json]
molecular version
```

The CLI currently targets an HTTP API at `http://127.0.0.1:8711`.
