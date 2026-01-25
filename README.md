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

## Local observability

This repo includes a local dev tracing stack (OpenTelemetry Collector + Jaeger) using Podman Quadlets (systemd-managed containers).

Jaeger runs as v2 (Jaeger v1 is EOL).

Images are pinned in `quadlets/` for repeatable local dev.

Quickstart:

```sh
mkdir -p "$HOME/.config/containers/systemd/molecular"
cp quadlets/* "$HOME/.config/containers/systemd/molecular/"
systemctl --user daemon-reload
systemctl --user start molecular-jaeger.service
systemctl --user start molecular-otel-collector.service
systemctl --user start molecular-tracegen.service
```

Jaeger UI: `http://127.0.0.1:16686`

Verify services are showing up:

```sh
curl -fsS http://127.0.0.1:16686/api/services
```

Full runbook: `docs/observability-local-dev.md`.

## Troubleshooting (quadlets)

Check which Molecular units are present and their state:

```sh
systemctl --user list-units --type=service | grep -E 'molecular-'
systemctl --user list-unit-files | grep -E '^molecular-'
podman ps --all | grep -E 'molecular-(jaeger|otel-collector)|telemetrygen'
```

If a unit failed, inspect status + logs:

```sh
systemctl --user status molecular-jaeger.service molecular-otel-collector.service
journalctl --user -u molecular-jaeger.service -n 200 --no-pager
journalctl --user -u molecular-otel-collector.service -n 200 --no-pager
```

Common fixes:

```sh
# Clear failed state and retry
systemctl --user reset-failed molecular-jaeger.service
systemctl --user start molecular-jaeger.service

# If you previously ran a compose stack, old containers may conflict with names
podman ps -a | grep -E '(^|[[:space:]])(jaeger|otel-collector)([[:space:]]|$)'
podman rm -f jaeger otel-collector 2>/dev/null || true

# Port conflicts (Jaeger UI)
ss -ltnp | grep -E ':16686'

# Podman image short-name prompts under systemd (non-interactive)
# If logs show: "short-name resolution enforced but cannot prompt without a TTY"
# ensure quadlets use fully-qualified image names (e.g. docker.io/...).

# If tracegen can't connect to the collector (connection refused)
# ensure the collector OTLP receiver binds to 0.0.0.0 (not 127.0.0.1) inside the container.

# If you can't exec into a container to debug
# some images don't ship with /bin/sh; use a debug container on the same network.
# Example:
# podman run --rm --network molecular-observability docker.io/busybox:latest sh -lc 'nc -zvw3 molecular-jaeger 4317'
```
