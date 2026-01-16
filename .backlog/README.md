# Molecular Backlog

This directory contains implementation-ready task tracking for Molecular features.

## Structure

```
.backlog/
├── README.md                          # This file
├── <feature_name>/
│   ├── index.md                       # Task table + feature description
│   └── mol-<abc>-NNN.md              # Individual task files
└── .archive/
    └── YYYYMMDD-<feature_name>/      # Archived completed features
```

## Task ID Format

`mol-<abc>-NNN`

- `mol-` prefix (molecular)
- `<abc>` exactly 3-letter feature acronym (lowercase)
- `-NNN` zero-padded task number (001, 002, etc.)

**Examples:**
- `mol-pte-001` - Prompt Template Engine, task 1
- `mol-ote-001` - OpenTelemetry, task 1
- `mol-his-001` - History command, task 1

## Task Status Values

- `todo` - Not started
- `in_progress` - Currently being worked on
- `blocked` - Cannot proceed (missing dependency, clarification needed)
- `done` - Completed
- `cancelled` - No longer needed

## Feature Lifecycle

1. **Create** - New feature folder with index.md and initial tasks
2. **Implement** - Work through tasks, updating status as you go
3. **Complete** - All tasks done, feature tested and merged
4. **Archive** - Move to `.archive/YYYYMMDD-<feature_name>/`

## Ownership

**Gommander** owns the backlog:
- Creates and maintains feature directories
- Writes all backlog documentation
- Keeps task status current
- Delegates implementation to specialized agents (gopher, builder, etc.)
- Archives completed features

## Creating a New Feature

Use the `backlog-create-feature` skill to ensure consistent structure.

## Archiving a Feature

When a feature is complete:
1. Rename folder with completion date: `YYYYMMDD-<feature_name>`
2. Move to `.archive/`
3. Update archived `index.md` with completion date and summary

Example:
```
.backlog/prompt-engine/  →  .backlog/.archive/20260117-prompt-engine/
```
