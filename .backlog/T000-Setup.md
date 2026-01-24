# T000 - Setup: Observability (OTel tracing) + Logging + Local Dev Stack

## Goal

Establish a first-class observability foundation for Molecular (starting with Silicon):

- **OpenTelemetry tracing** (official OTel Go packages)
- **slog** for traditional logs (only for lifecycle / non-trace-worthy events)
- A **podman compose** stack that spins up **OpenTelemetry Collector + Jaeger** for local development

## Primary principles

1. **Each Silicon task == one trace** (root span per task).
2. Use **spans + span events** to tell the execution story.
   - Events should record milestones, decisions, retries, and relevant metadata.
3. Use **plain logs sparingly** (startup/shutdown/config errors, fatal conditions where traces aren’t available).
4. Prefer **context propagation** over globals so spans/events are attached to the correct trace.
5. **Fail fast**: if the tracer/exporter cannot be initialized, Silicon should **exit immediately**.

## In scope

### 1) Tracing architecture

- Introduce an `internal/telemetry` (or `internal/otel`) package responsible for:
  - creating the OTel `TracerProvider`
  - setting global propagators
  - setting resource attributes (service name, version)
  - providing a `Shutdown(ctx)` function to flush exporters

- Silicon should:
  - initialize tracing once at startup
  - crash/exit if tracing init fails
  - create a root span per task execution
  - create child spans for each major step (HTTP handling, validation, executor run, cancellation)
  - emit **span events** for high-signal story points

### 2) Logging

- Standard library logger should be **`log/slog`**.
- Logging usage guidance:
  - `INFO` startup/shutdown
  - `WARN/ERROR` fatal configuration or exporter issues
  - no chatty “workflow logs” — those should become span events

### 3) Local dev stack (podman compose)

Add a compose file (for podman) that starts:

- **otel-collector** (latest)
  - OTLP receiver enabled (grpc + http)
  - exports traces to Jaeger
- **jaeger** (latest)
  - exposes UI port

Also include a collector config file in-repo.

## Proposed trace model (v1)

### Root span

- Name: `task` (or `silicon.task`)
- Attributes:
  - `task.id`
  - `task.prompt_len`
  - `service.version` (from `internal/version`)

### Naming + semantic conventions (enforced)

We will enforce consistent naming across traces/spans/events using OTel semantic conventions where applicable.

When possible, use:

- https://pkg.go.dev/go.opentelemetry.io/otel/semconv/v1.39.0

#### Project attribute namespace (stable)

In addition to semconv, we will define and **keep stable** our own attribute namespaces:

- `silicon.*` (daemon/orchestrator)
- future: `carbon.*`, `helium.*`, `lithium.*`, `chlorine.*`

These should not churn when semconv versions update.

#### Span names

- `silicon.http.request` (HTTP entry)
- `silicon.task` (root span per task)
- `silicon.task.execute` (executor)
- Future step spans: `silicon.task.lithium`, `silicon.task.carbon`, `silicon.task.helium`, `silicon.task.chlorine`

#### Event names

- `task.created`
- `task.started`
- `task.completed`
- `task.failed`
- `task.cancelled`

Use attributes for detail (avoid encoding details into event names).

### Recommended child spans

- `http.POST /v1/tasks` (request handling / validation)
- `task.execute` (top-level executor)
- Later (future tasks): `task.lithium`, `task.carbon`, `task.helium`, `task.chlorine`

### Recommended span events

- `task.created`
- `task.started`
- `task.completed` / `task.failed` / `task.cancelled`
- `decision.*` (later: Helium decision)
- `retry.*` (later: budget/retry decisions)

## Configuration (initial)

Minimum env-based configuration for v1:

- `OTEL_EXPORTER_OTLP_ENDPOINT` (default: `http://127.0.0.1:4318`)
- `OTEL_EXPORTER_OTLP_PROTOCOL` (optional, default `http/protobuf`)
- `OTEL_SERVICE_NAME` (default: `molecular-silicon`)
- `OTEL_TRACES_SAMPLER` (default: `parentbased_always_on` or `always_on` for dev)

No config file parsing required for T000.

OTLP transport choice (v1):

- Prefer **OTLP/HTTP (4318)** for simplicity (dev-first; no perf chasing).

### .env support (local dev)

For local development, Silicon should support loading environment variables from a `.env` file using:

- https://github.com/joho/godotenv

Guidance:

- Call `godotenv.Load()` at process startup (before telemetry init).
- Missing `.env` should be treated as normal (no error).
- Parse errors should be surfaced (slog warn) since traces may not be initialized yet.

## Decisions (confirmed)

- Compose filename: `compose.yml`
- Jaeger: **all-in-one** (dev)
- Sampling: **always on**
- HTTP instrumentation: `otelhttp` is acceptable
- Scope: **traces only** (no metrics)

## Deliverables

1. Podman compose file + collector config + brief instructions (in README or docs).
2. Telemetry init package with clean API.
3. Silicon integrated with tracing + slog.
4. At least one test that asserts:
   - a task execution creates spans (using an in-memory exporter)

## Dependencies policy

This setup will introduce **official OTel Go packages**, likely including:

- `go.opentelemetry.io/otel`
- `go.opentelemetry.io/otel/sdk`
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace` (+ http/grpc subpackages)
- `go.opentelemetry.io/otel/propagation`
- `go.opentelemetry.io/otel/semconv`

And for `.env` handling:

- `github.com/joho/godotenv`

**Open question:** is `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` acceptable? It’s in opentelemetry-go-contrib (official org, but “contrib”). If not, we’ll manually instrument HTTP handlers.

## Subtasks (each on its own feature branch)

### T000-1: Local dev tracing stack (podman compose + collector config)

Branch: `feature/T000-1-otel-dev-stack`

Deliverables:

- `compose.yml` (or `podman-compose.yml`) with `otel-collector` + `jaeger`
- `otel-collector-config.yaml`
- Short runbook:
  - how to start stack
  - where to access Jaeger UI
  - what endpoint Silicon should export to

Acceptance criteria:

- `podman compose up` starts both services
- Jaeger UI reachable
- Collector listens on OTLP (4317/4318)

### T000-2: Telemetry package (OTel init + slog guidance)

Branch: `feature/T000-2-otel-init`

Deliverables:

- `internal/telemetry` package with:

```go
type Config struct {
    ServiceName string
    ServiceVersion string
    OTLPEndpoint string
    Insecure bool // for local
}

func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error)
```

- Uses OTel SDK and OTLP exporter.

Acceptance criteria:

- No new non-OTel dependencies
- `go test ./...` passes

Failure behavior:

- If telemetry init fails, Silicon should `slog.Error(...)` and exit with non-zero status.

Additional acceptance criteria:

- `cmd/silicon` loads `.env` via `godotenv.Load()` (best-effort)

### T000-3: Silicon tracing integration (task = trace)

Branch: `feature/T000-3-silicon-tracing`

Deliverables:

- On task creation/execution:
  - create root span per task
  - add events for state transitions
  - ensure executor runs with the trace context
- Add an in-memory exporter test:
  - submit task
  - assert span(s) were emitted

Acceptance criteria:

- Each task produces a trace in Jaeger (manual verification)
- Test verifies spans exist
- Plain logs limited to startup/shutdown

### T000-4 (recommended): Graceful shutdown + flush

Branch: `feature/T000-4-graceful-shutdown`

Rationale: tracing only helps if we reliably flush spans on exit.

Deliverables:

- `cmd/silicon` handles SIGINT/SIGTERM
- Calls `http.Server.Shutdown(ctx)`
- Calls telemetry shutdown (flush) with timeout
- Uses `slog` for start/stop messages

Acceptance criteria:

- On SIGINT, Silicon exits cleanly and traces appear in Jaeger

### T000-5 (optional): Trace readability improvements

Branch: `feature/T000-5-trace-readability`

Deliverables:

- Add consistent span attributes and events naming conventions
- Ensure errors are recorded with `span.RecordError(err)` and `span.SetStatus(...)`
- Add correlation between slog and trace (include `trace_id` in startup log where possible)

### T000-6 (recommended): HTTP server hardening (timeouts, limits, recovery)

Branch: `feature/T000-6-http-hardening`

Deliverables:

- Wrap the Silicon HTTP server with sensible defaults:
  - `ReadHeaderTimeout`
  - `ReadTimeout`
  - `WriteTimeout`
  - `IdleTimeout`
  - `MaxHeaderBytes`
- Add request body size limits for JSON endpoints (e.g. `http.MaxBytesReader`).
- Add a panic recovery middleware/handler wrapper:
  - Returns `500`.
  - Records panic as a span event and sets span status/error.
  - Uses `slog.Error` only for the panic + server-level failure cases.

Acceptance criteria:

- Unit test for body size limit returns `413` (or chosen status) for oversized payload.
- Unit test that a handler panic returns `500` and still produces a span.

## Open questions / follow-ups

1. Do you want a standard schema for event names and attributes (e.g. `task.*`, `http.*`, `executor.*`) enforced early?
   - **Answer:** Yes. Enforce standard naming (see "Naming + semantic conventions" above).
2. Do you want to keep completed tasks in memory forever, or add an eviction policy later?
   - **Answer:** Keep finished tasks in memory until we add SQLite backend.
