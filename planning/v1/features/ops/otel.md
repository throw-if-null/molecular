# Feature: OpenTelemetry (Structured Logging + Tracing)

## Goal

Add OpenTelemetry (OTel) instrumentation to Molecular for structured logging and distributed tracing, enabling observability via tools like Jaeger, Grafana Tempo, or similar OTLP-compatible backends.

## Current state

- Logging uses Go's standard `log` package with unstructured output
- No tracing infrastructure
- Difficult to track task/attempt flow through worker loops
- No correlation between logs and task/attempt lifecycle

## Requirements

### Always-on observability

- OTel is **always enabled** (not optional)
- Minimal performance impact (async export, sampling for high-volume ops)
- Graceful degradation if OTel backend unavailable

### Structured logging

- Replace `log.*` calls with `slog` (Go 1.21+ structured logging)
- Log levels: DEBUG, INFO, WARN, ERROR
- Contextual fields: task_id, attempt_id, role, phase
- Use logs for startup, shutdown, cleanup, panic recovery

### Distributed tracing

- Trace spans for:
  - HTTP request handlers
  - Worker poll loops (one span per iteration)
  - Attempt lifecycle (setup → execute → write artifacts)
  - Store operations (CreateAttempt, UpdateTaskPhase, etc.)
  - External command execution (Carbon/Helium/Chlorine commands)
- Span attributes: task_id, attempt_id, role, phase, status, error
- Trace events for significant milestones (attempt started, command succeeded, etc.)

### Configuration

Add `[otel]` section to `.molecular/config.toml`:

```toml
[otel]
enabled = true                              # Always true in practice
endpoint = "http://localhost:4318"          # OTLP HTTP endpoint
service_name = "molecular-silicon"
environment = "development"                 # e.g., development, staging, production
```

### Dependencies

New dependencies (approved):
- `go.opentelemetry.io/otel`
- `go.opentelemetry.io/otel/sdk`
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp`
- `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp` (for log export)
- `go.opentelemetry.io/otel/sdk/log`
- `go.opentelemetry.io/otel/sdk/trace`

## Detailed implementation steps

### 1. Add dependencies

**File:** `go.mod`

**Commands:**
```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/sdk
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
go get go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp
go get go.opentelemetry.io/otel/sdk/log
go get go.opentelemetry.io/otel/sdk/trace
go mod tidy
```

### 2. Create `internal/otel` package

**File:** `internal/otel/otel.go`

**Responsibilities:**
- Initialize OTel SDK (tracer provider, logger provider)
- Configure OTLP exporters (HTTP)
- Set global tracer and logger
- Graceful shutdown on signal

**API:**
```go
package otel

import (
    "context"
    "log/slog"
    "go.opentelemetry.io/otel/trace"
)

type Config struct {
    Enabled     bool
    Endpoint    string
    ServiceName string
    Environment string
}

// Init initializes OTel SDK with configured exporters
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error)

// Tracer returns the global tracer
func Tracer(name string) trace.Tracer

// Logger returns the global structured logger
func Logger() *slog.Logger
```

**Implementation notes:**
- Use `otlptracehttp.New()` for trace exporter
- Use `otlploghttp.New()` for log exporter
- Set resource attributes: service.name, service.version, deployment.environment
- Configure batch span processor for async export
- Configure batch log processor for async export
- Handle exporter errors gracefully (log to stderr, continue operation)

### 3. Update config schema

**File:** `internal/config/config.go`

**Add OTel section:**
```go
type Config struct {
    Silicon SiliconConfig `toml:"silicon"`
    Retry   RetryConfig   `toml:"retry"`
    Workers WorkersConfig `toml:"workers"`
    Hooks   HooksConfig   `toml:"hooks"`
    OTel    OTelConfig    `toml:"otel"`
}

type OTelConfig struct {
    Enabled     bool   `toml:"enabled"`
    Endpoint    string `toml:"endpoint"`
    ServiceName string `toml:"service_name"`
    Environment string `toml:"environment"`
}
```

**Default values:**
```go
OTel: OTelConfig{
    Enabled:     true,
    Endpoint:    "http://localhost:4318",
    ServiceName: "molecular-silicon",
    Environment: "development",
}
```

### 4. Initialize OTel in Silicon

**File:** `cmd/silicon/main.go`

**Changes:**
```go
import "github.com/throw-if-null/molecular/internal/otel"

func main() {
    // Load config
    cfg := config.Load(repoRoot)
    
    // Initialize OTel
    shutdown, err := otel.Init(ctx, cfg.Config.OTel)
    if err != nil {
        log.Fatalf("failed to initialize otel: %v", err)
    }
    defer shutdown(context.Background())
    
    // Use structured logger
    logger := otel.Logger()
    logger.Info("silicon starting", "version", version.Version, "repo_root", repoRoot)
    
    // Start workers...
}
```

### 5. Instrument HTTP handlers

**File:** `internal/silicon/server.go`

**Changes:**
- Wrap handlers with tracing middleware
- Extract trace context from HTTP headers
- Add span for each request
- Log request/response with trace_id

**Example:**
```go
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
    ctx, span := otel.Tracer("silicon").Start(r.Context(), "GET /v1/tasks/{task_id}")
    defer span.End()
    
    taskID := r.PathValue("task_id")
    span.SetAttributes(attribute.String("task_id", taskID))
    
    task, err := s.store.GetTask(taskID)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    
    span.AddEvent("task retrieved", trace.WithAttributes(
        attribute.String("phase", task.Phase),
        attribute.String("status", task.Status),
    ))
    
    json.NewEncoder(w).Encode(task)
}
```

### 6. Instrument worker loops

**File:** `internal/silicon/worker.go`

**Changes:**
- Add span for each worker poll iteration
- Add span for attempt lifecycle
- Use span attributes for task/attempt metadata
- Use trace events for milestones

**Example (Carbon worker):**
```go
func StartCarbonWorker(...) {
    logger := otel.Logger()
    tracer := otel.Tracer("silicon.carbon")
    
    for {
        select {
        case <-ticker.C:
            ctx, span := tracer.Start(ctx, "carbon.poll")
            
            tasks, err := s.ListTasks(0)
            if err != nil {
                span.RecordError(err)
                span.End()
                continue
            }
            
            for _, t := range tasks {
                if t.Phase != "build" || t.Status != "running" {
                    continue
                }
                
                // Create attempt span
                attemptCtx, attemptSpan := tracer.Start(ctx, "carbon.attempt")
                attemptSpan.SetAttributes(
                    attribute.String("task_id", t.TaskID),
                    attribute.String("role", "carbon"),
                )
                
                logger.Info("starting carbon attempt",
                    "task_id", t.TaskID,
                    "trace_id", attemptSpan.SpanContext().TraceID().String(),
                )
                
                // Execute attempt...
                
                attemptSpan.AddEvent("command started")
                // Run command...
                attemptSpan.AddEvent("command finished", trace.WithAttributes(
                    attribute.Int("exit_code", exitCode),
                ))
                
                attemptSpan.End()
            }
            
            span.End()
        }
    }
}
```

### 7. Instrument store operations

**File:** `internal/store/store.go`

**Changes:**
- Add span for each store operation
- Use span attributes for query parameters
- Log slow queries (>100ms)

**Example:**
```go
func (s *Store) CreateAttempt(taskID, role string) (..., error) {
    ctx, span := otel.Tracer("silicon.store").Start(context.Background(), "CreateAttempt")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("task_id", taskID),
        attribute.String("role", role),
    )
    
    // Execute SQL...
    
    if err != nil {
        span.RecordError(err)
        return "", "", "", time.Time{}, err
    }
    
    span.AddEvent("attempt created", trace.WithAttributes(
        attribute.String("attempt_id", attemptID),
    ))
    
    return attemptID, artifactsDir, session, startedAt, nil
}
```

### 8. Replace unstructured logging

**Throughout codebase:**

Replace:
```go
log.Printf("starting carbon worker")
```

With:
```go
otel.Logger().Info("starting carbon worker", "role", "carbon")
```

**Use cases for logging (not tracing):**
- Startup messages (Silicon binding to port, config loaded)
- Shutdown messages (workers stopped, cleanup complete)
- Panic recovery (stack trace, error context)
- Configuration validation errors

**Use cases for tracing:**
- Request/response flow
- Worker loops
- Attempt lifecycle
- Command execution
- Store operations

**Use cases for trace events:**
- Milestones within spans (command started, artifact written)
- State transitions (lithium → build, build → review)
- Retry attempts

### 9. Update documentation

**File:** `README.md`

**Add observability section:**
```markdown
## Observability

Molecular uses OpenTelemetry for structured logging and distributed tracing.

### Local Jaeger setup

Run Jaeger locally to visualize traces:

\```bash
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4318:4318 \
  jaegertracing/all-in-one:latest
\```

Configure Molecular:

\```toml
[otel]
enabled = true
endpoint = "http://localhost:4318"
service_name = "molecular-silicon"
\```

View traces at http://localhost:16686
```

### 10. Tests

**Unit tests:**
- OTel initialization succeeds
- OTel initialization with invalid endpoint (graceful fallback)
- Span creation and attributes
- Logger output format

**Integration tests:**
- Submit task, verify traces exported
- Verify trace context propagation across spans
- Verify attempt spans include task_id attribute

## Acceptance criteria

- [ ] OTel SDK initialized on Silicon startup
- [ ] HTTP handlers instrumented with tracing
- [ ] Worker loops instrumented with tracing
- [ ] Store operations instrumented with tracing
- [ ] Structured logging replaces `log.*` calls
- [ ] Trace events used for milestones
- [ ] Configuration supports custom OTLP endpoint
- [ ] Documentation includes Jaeger setup guide
- [ ] Graceful degradation if OTel backend unavailable
- [ ] All tests passing

## Example trace hierarchy

```
Trace: submit-task-feat-123
│
├─ Span: POST /v1/tasks (2.3s)
│  ├─ Event: task created
│  └─ Attributes: task_id=feat-123, prompt="..."
│
├─ Span: lithium.poll (50ms)
│  └─ Span: lithium.attempt (2.1s)
│     ├─ Event: worktree ensured
│     ├─ Event: hook executed
│     └─ Attributes: task_id=feat-123, role=lithium, status=success
│
├─ Span: carbon.poll (120ms)
│  └─ Span: carbon.attempt (45.2s)
│     ├─ Event: command started
│     ├─ Event: command finished (exit_code=0)
│     ├─ Event: result.json written
│     └─ Attributes: task_id=feat-123, role=carbon, status=success
│
├─ Span: helium.poll (80ms)
│  └─ Span: helium.attempt (12.5s)
│     ├─ Event: command started
│     ├─ Event: decision parsed (approved)
│     └─ Attributes: task_id=feat-123, role=helium, decision=approved
│
└─ Span: chlorine.poll (60ms)
   └─ Span: chlorine.attempt (5.2s)
      ├─ Event: branch created
      ├─ Event: pr created
      └─ Attributes: task_id=feat-123, role=chlorine, pr_url="..."
```

## Follow-up work (post-v1)

- Add metrics (task throughput, attempt duration, error rates)
- Sampling configuration (sample high-volume ops)
- Custom OTel exporters (stdout, file)
- Trace visualization in `molecular` CLI
- Correlation of logs with traces via trace_id
