# v1 planning

This directory tracks the work needed to get to a real v1 product.

## Feature index

See `planning/v1/features/README.md` for the complete list.

### Active features (v1.1)

**Templates:**
- `planning/v1/features/templates/README.md` - Prompt templates + skills system

**CLI:**
- `planning/v1/features/cli/history.md` - `molecular history` command
- `planning/v1/features/cli/completions.md` - Bash completions
- `planning/v1/features/cli/watch-mode.md` - `--watch` mode for status

**Operations:**
- `planning/v1/features/ops/otel.md` - OpenTelemetry (logging + tracing)

### Completed features (v1.0)

**Configuration:**
- `planning/v1/features/config/README.md` ✓

**API:**
- Logs endpoint ✓

**Workers:**
- Carbon (builder) execution ✓
- Helium (inspector) execution ✓
- Chlorine (PR creation) ✓

**CLI:**
- Status UX improvements ✓
- Cleanup command ✓

**Operations:**
- Cancellation semantics ✓
- Crash recovery ✓
- Security (path traversal prevention) ✓

## Workflow

We use feature branches for development (not git worktrees). Molecular’s own runtime model still uses git worktrees per task.

Suggested branch naming:
- `feature/config`
- `feature/logs-endpoint`
- `feature/carbon-exec`
- `feature/helium-exec`
- `feature/pr-creation`
