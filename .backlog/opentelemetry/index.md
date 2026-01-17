# OpenTelemetry

## Overview

Add OpenTelemetry (OTel) instrumentation to Molecular for structured logging and distributed tracing, enabling observability via tools like Jaeger, Grafana Tempo, or similar OTLP-compatible backends.

See detailed specification: [`planning/v1/features/ops/otel.md`](../../planning/v1/features/ops/otel.md)

## Goals

- Replace unstructured logging with `slog`
- Add distributed tracing for task/attempt lifecycle
- Instrument HTTP handlers, workers, and store operations
- Minimal performance impact with graceful degradation

## Implementation Approach

1. Add OTel dependencies (requires user approval per dependency policy)
2. Create `internal/otel` package for SDK initialization
3. Replace `log.*` calls with `slog`
4. Instrument HTTP handlers with tracing middleware
5. Instrument worker loops and store operations
6. Add configuration support via `.molecular/config.toml`

## Tasks

| ID | Title | Status | Dependencies | Notes |
|----|-------|--------|--------------|-------|
| [mol-ote-001](./mol-ote-001.md) | Add OTel dependencies | todo | - | Requires user approval |
| [mol-ote-002](./mol-ote-002.md) | Create internal/otel package | todo | mol-ote-001 | SDK initialization |
| [mol-ote-003](./mol-ote-003.md) | Add config schema for OTel | todo | mol-ote-002 | [otel] section in config.toml |
| [mol-ote-004](./mol-ote-004.md) | Replace log.* with slog | todo | mol-ote-002 | Structured logging |
| [mol-ote-005](./mol-ote-005.md) | Instrument HTTP handlers | todo | mol-ote-002 | Tracing middleware |
| [mol-ote-006](./mol-ote-006.md) | Instrument worker loops | todo | mol-ote-002 | Span per iteration |
| [mol-ote-007](./mol-ote-007.md) | Instrument store operations | todo | mol-ote-002 | Query tracing |
| [mol-ote-008](./mol-ote-008.md) | Add tests and documentation | todo | mol-ote-004, mol-ote-005, mol-ote-006, mol-ote-007 | Jaeger setup guide |

### Task Dependency Graph

```
mol-ote-001 (dependencies)
    └─> mol-ote-002 (internal/otel package)
            ├─> mol-ote-003 (config schema)
            ├─> mol-ote-004 (slog migration) ──┐
            ├─> mol-ote-005 (HTTP tracing) ────┼─> mol-ote-008 (tests + docs)
            ├─> mol-ote-006 (worker tracing) ──┤
            └─> mol-ote-007 (store tracing) ───┘
```

## Feature Dependencies

- Requires new external dependencies (OTel Go SDK packages)
- User must approve dependency additions per gommander policy

## Testing Strategy

- Unit tests for OTel initialization
- Unit tests for graceful degradation (backend unavailable)
- Integration test: submit task, verify traces exported
- Manual test with local Jaeger instance
- Verify trace context propagation

## Documentation Updates

- Add "Observability" section to README.md
- Document Jaeger local setup
- Document configuration options
- Add troubleshooting guide

## Acceptance Criteria

- [ ] OTel SDK initialized on Silicon startup
- [ ] HTTP handlers instrumented with tracing
- [ ] Worker loops instrumented with tracing
- [ ] Store operations instrumented with tracing
- [ ] Structured logging replaces `log.*` calls
- [ ] Trace events for milestones
- [ ] Configuration via `.molecular/config.toml`
- [ ] Graceful degradation if backend unavailable
- [ ] Documentation includes Jaeger setup guide
- [ ] All tests passing
