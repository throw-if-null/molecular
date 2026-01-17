# Molecular Backlog

This file tracks all features in active development and provides quick access to their detailed task breakdowns.

For detailed information about the backlog system, see [`.backlog/README.md`](.backlog/README.md).

## Active Features

| Feature | Status | Tasks | Description | Link |
|---------|--------|-------|-------------|------|
| Prompt Template Engine | todo | 10 | Generic template system for Carbon/Helium with variable substitution and skill composition | [Details](.backlog/prompt-engine/index.md) |
| History Command | todo | 4 | View all attempts for a task with metadata (role, status, duration, errors) | [Details](.backlog/history-command/index.md) |
| Bash Completions | todo | 3 | Tab-completion for molecular CLI commands, flags, and task IDs | [Details](.backlog/completions/index.md) |
| Watch Mode | todo | 4 | Real-time status monitoring with auto-refresh and phase transitions | [Details](.backlog/watch-mode/index.md) |
| OpenTelemetry | todo | 8 | Structured logging and distributed tracing for observability | [Details](.backlog/opentelemetry/index.md) |

## Archived Features

Completed features are archived to `.backlog/.archive/YYYYMMDD-<feature_name>/`.

Currently no archived features.

---

## Feature Status Legend

- **todo** - Not started, ready for implementation
- **in_progress** - Actively being worked on
- **blocked** - Waiting on dependency or decision
- **done** - Completed, ready for archive

## Adding a New Feature

Use the `backlog-create-feature` skill to create a new feature with proper structure:
1. Create feature directory in `.backlog/<feature_name>/`
2. Create `index.md` with task table
3. Create individual task files (`mol-<abc>-NNN.md`)
4. Add entry to this file

## Quick Links

- [Backlog README](.backlog/README.md) - System documentation
- [Backlog Skills](.opencode/skill/) - Management procedures
- [Archive](.backlog/.archive/) - Completed features
