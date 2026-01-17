# Bash Completions

## Overview

Add bash completion support for the `molecular` CLI to improve developer experience with tab-completion for commands, flags, and task IDs.

See detailed specification: [`planning/v1/features/cli/completions.md`](../../planning/v1/features/cli/completions.md)

## Goals

- Tab-complete commands (submit, status, list, cancel, logs, cleanup, history, doctor, version)
- Tab-complete flags (--task-id, --prompt, --json, --tail, --watch, --interval, --limit)
- Dynamic task ID completion (queries `molecular list` for active tasks)
- Easy installation for users (system-wide, user-local, or sourced in .bashrc)

## Implementation Approach

1. Create bash completion script using bash-completion conventions
2. Implement command completion with `compgen`
3. Implement per-command flag completion
4. Add dynamic task ID completion via `molecular list` subshell call
5. Document installation methods and test manually

## Tasks

| ID | Title | Status | Dependencies | Notes |
|----|-------|--------|--------------|-------|
| [mol-cmp-001](./mol-cmp-001.md) | Create completion script structure | todo | - | scripts/completions/bash/molecular |
| [mol-cmp-002](./mol-cmp-002.md) | Add command completion | todo | mol-cmp-001 | compgen with command list |
| [mol-cmp-003](./mol-cmp-003.md) | Add per-command flag completion | todo | mol-cmp-001 | Case statement per command |
| [mol-cmp-004](./mol-cmp-004.md) | Add dynamic task ID completion | todo | mol-cmp-003 | Subshell: molecular list |
| [mol-cmp-005](./mol-cmp-005.md) | Test and document installation | todo | mol-cmp-004 | README + manual testing |

### Task Dependency Graph

```
mol-cmp-001 (script structure)
    ├─> mol-cmp-002 (command completion)
    └─> mol-cmp-003 (flag completion)
            └─> mol-cmp-004 (task ID completion)
                    └─> mol-cmp-005 (test + docs)
```

## Feature Dependencies

None - standalone shell script. Requires bash-completion package (standard on most systems).

## Testing Strategy

- **Manual testing with bash:**
  - Test command completion: `molecular <TAB>` shows all commands
  - Test partial command: `molecular st<TAB>` completes to `status`
  - Test flag completion: `molecular status --<TAB>` shows `--json`, `--watch`
  - Test task ID completion: `molecular status <TAB>` shows active task IDs (requires Silicon running)
  - Test with no running Silicon (should not error, just no task completions)
- **Installation methods:**
  - System-wide: `/etc/bash_completion.d/`
  - User-local: `~/.local/share/bash-completion/completions/`
  - Direct source: Add to `.bashrc`
- **Compatibility:**
  - Test with bash 4.0+ (standard bash-completion features)
  - Verify no external dependencies beyond bash-completion

## Documentation Updates

- Add "Shell Completions" section to README.md
- Document all three installation methods with examples
- Add troubleshooting tips (e.g., "completions not working? check bash-completion installed")
- Mention that task ID completion requires Silicon to be running

## Acceptance Criteria

- [ ] Commands autocomplete with TAB (all 9 commands)
- [ ] Partial commands autocomplete (e.g., `st<TAB>` → `status`)
- [ ] Flags autocomplete per command (e.g., `status --<TAB>` shows `--json`, `--watch`)
- [ ] Task IDs autocomplete dynamically when Silicon is running
- [ ] Script follows bash-completion conventions (`_init_completion`, `compgen`)
- [ ] Works with bash 4.0+ (no bash 5-specific features)
- [ ] No errors when Silicon is not running (graceful degradation)
- [ ] Installation documented in README with all three methods
- [ ] Script is executable: `chmod +x scripts/completions/bash/molecular`
- [ ] No external dependencies beyond bash-completion package
