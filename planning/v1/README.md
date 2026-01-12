# v1 planning

This directory tracks the work needed to get to a real v1 product.

## Feature index

### Configuration
- `planning/v1/features/config/README.md`

### API
- `planning/v1/features/api/logs-endpoint.md`

### Workers
- `planning/v1/features/workers/carbon-exec.md`
- `planning/v1/features/workers/helium-exec.md`
- `planning/v1/features/workers/chlorine-pr.md`

### CLI
- `planning/v1/features/cli/status-ux.md`
- `planning/v1/features/cli/cleanup.md`

### Ops
- `planning/v1/features/ops/cancellation.md`
- `planning/v1/features/ops/crash-recovery.md`
- `planning/v1/features/ops/security.md`

## Workflow

We use feature branches for development (not git worktrees). Molecular’s own runtime model still uses git worktrees per task.

Suggested branch naming:
- `feature/config`
- `feature/logs-endpoint`
- `feature/carbon-exec`
- `feature/helium-exec`
- `feature/pr-creation`
