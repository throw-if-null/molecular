# v2 planning

This directory tracks features planned for Molecular v2.

## Feature index

### Workers
- `planning/v2/features/workers/phosphorus.md` - Reporter worker (HTML reports)

### Operations
- `planning/v2/features/ops/monitoring.md` - Health monitoring and alerting hooks

## Philosophy

v2 focuses on **operability and shareability**:
- Make task results easy to share with team members (Phosphorus reports)
- Make Silicon easy to monitor in production (health metrics, alerting)
- Enhance developer experience with polished UX

## Prerequisites

v2 features depend on v1 completion:
- Stable worker execution (Carbon, Helium, Chlorine)
- OpenTelemetry instrumentation (for monitoring)
- Artifact storage structure (for report generation)

## Implementation approach

v2 features should:
- Be implemented on feature branches
- Follow the same development workflow as v1
- Include comprehensive tests
- Update documentation
- Archive specs after implementation

## Roadmap

**Target:** Q2 2026

1. **Phosphorus reporter worker** (4-6 weeks)
   - HTML report generation
   - Artifact bundling
   - Optional upload to remote storage

2. **Health monitoring + alerting** (2-3 weeks)
   - Enhanced `/healthz` endpoint
   - Webhook alerts for failures/stuck tasks
   - CLI health command

## Nice-to-haves (v2.1+)

- Metrics dashboard (aggregate stats across tasks)
- Slack/Teams bot integration
- Report templates (customizable HTML/CSS)
- PDF report export
- Multi-repo support (one Silicon managing multiple repos)
