# Prompt Template Engine

## Overview

Implement a generic prompt template system for Carbon (builder) and Helium (inspector) agents that allows repository-specific customization through Markdown templates with variable substitution and skill composition.

This system replaces the `.foreman/` Python-based template system with a Go-native, file-based approach that is generic by default but customizable per repository.

## Goals

- Provide mandatory prompt templates for Carbon and Helium agents
- Support variable substitution (`{{variable}}`) and skill composition (`{{skill:name}}`)
- Make templates generic (not bound to specific tech stacks like pnpm/Svelte)
- Enable repository-specific customization through `.molecular/<agent>/` directories
- Validate templates at startup via `molecular doctor`

## Implementation Approach

1. Create `internal/templates` package for loading and rendering templates
2. Define template structure: `.molecular/<agent>/prompt.md` + `.molecular/<agent>/skills/*.md`
3. Implement variable substitution and recursive skill rendering
4. Create generic base templates for Carbon and Helium
5. Integrate template rendering into worker execution paths
6. Add template validation to `molecular doctor`
7. Migrate existing `.foreman/` templates as examples

## Tasks

| ID | Title | Status | Dependencies | Notes |
|----|-------|--------|--------------|-------|
| [mol-pte-001](./mol-pte-001.md) | Create internal/templates package structure | todo | - | Foundation task |
| [mol-pte-002](./mol-pte-002.md) | Implement template Load function | todo | mol-pte-001 | |
| [mol-pte-003](./mol-pte-003.md) | Implement template Render function | todo | mol-pte-002 | |
| [mol-pte-004](./mol-pte-004.md) | Add template unit tests | todo | mol-pte-003 | |
| [mol-pte-005](./mol-pte-005.md) | Create generic Carbon base template | todo | - | Independent task |
| [mol-pte-006](./mol-pte-006.md) | Create generic Helium base template | todo | - | Independent task |
| [mol-pte-007](./mol-pte-007.md) | Integrate templates into Carbon worker | todo | mol-pte-003, mol-pte-005 | |
| [mol-pte-008](./mol-pte-008.md) | Integrate templates into Helium worker | todo | mol-pte-003, mol-pte-006 | |
| [mol-pte-009](./mol-pte-009.md) | Add template validation to molecular doctor | todo | mol-pte-003, mol-pte-005, mol-pte-006 | |
| [mol-pte-010](./mol-pte-010.md) | Migrate .foreman templates to .molecular | todo | mol-pte-007, mol-pte-008, mol-pte-009 | Final integration |

### Task Dependency Graph

```
mol-pte-001 (package structure)
    └─> mol-pte-002 (Load function)
            └─> mol-pte-003 (Render function)
                    ├─> mol-pte-004 (unit tests)
                    ├─> mol-pte-007 (Carbon integration) ──┐
                    ├─> mol-pte-008 (Helium integration) ──┼─> mol-pte-010 (migration)
                    └─> mol-pte-009 (doctor validation) ───┘

mol-pte-005 (Carbon template)
    └─> mol-pte-007 (Carbon integration)
    └─> mol-pte-009 (doctor validation)

mol-pte-006 (Helium template)
    └─> mol-pte-008 (Helium integration)
    └─> mol-pte-009 (doctor validation)
```

## Dependencies

- Existing worker execution paths (Carbon, Helium)
- `molecular doctor` command
- Config system (for potential template-related settings)

## Testing Strategy

- Unit tests for template loading, variable substitution, skill rendering
- Unit tests for circular skill detection and error handling
- Integration tests for template rendering in worker contexts
- Manual verification with `molecular doctor` template validation
- End-to-end test with migrated templates from `.foreman/`

## Documentation Updates

- Add "Templates" section to main README.md
- Create template authoring guide (possibly in `.backlog/.archive/` or dedicated docs/)
- Update `molecular doctor` help text to mention template validation
- Document template variable reference

## Acceptance Criteria

- [ ] Templates are mandatory for worker execution (fail gracefully if missing)
- [ ] Variable substitution works correctly (task_id, task_prompt, etc.)
- [ ] Skill composition works with recursive rendering
- [ ] Circular skill references are detected and reported
- [ ] Carbon worker renders templates before command execution
- [ ] Helium worker renders templates before command execution
- [ ] `molecular doctor` validates template presence and syntax
- [ ] Generic base templates exist for Carbon and Helium
- [ ] `.foreman/` templates migrated to `.molecular/` as examples
- [ ] All tests passing (`go test ./internal/templates/...`)
