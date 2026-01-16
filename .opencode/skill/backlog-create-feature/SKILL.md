---
name: backlog-create-feature
description: Create a new feature in the backlog with proper structure
---

## Purpose
Create a new feature folder in `.backlog/` with index.md and initial task files.

## Prerequisites
- Feature name decided (lowercase, hyphen-separated if multi-word)
- Feature acronym decided (exactly 3 lowercase letters)
- Initial task breakdown identified

## Procedure

1. **Create feature directory**
   ```
   .backlog/<feature_name>/
   ```

2. **Create index.md with this structure:**
   ```markdown
   # <Feature Title>
   
   ## Overview
   [Brief description of the feature - what it does and why]
   
   ## Goals
   - [Goal 1]
   - [Goal 2]
   
   ## Implementation Approach
   [High-level approach, architecture decisions, key design choices]
   
   ## Tasks
   
   | ID | Title | Status | Dependencies | Notes |
   |----|-------|--------|--------------|-------|
   | [mol-<abc>-001](./mol-<abc>-001.md) | Task title | todo | - | Foundation task |
   | [mol-<abc>-002](./mol-<abc>-002.md) | Task title | todo | mol-<abc>-001 | |
   | [mol-<abc>-003](./mol-<abc>-003.md) | Task title | todo | mol-<abc>-001, mol-<abc>-002 | |
   
   ### Task Dependency Graph
   
   ```
   mol-<abc>-001 (description)
       └─> mol-<abc>-002 (description)
               └─> mol-<abc>-003 (description)
   
   mol-<abc>-004 (independent task)
       └─> mol-<abc>-005 (description)
   ```
   
   ## Feature Dependencies
   - [List any dependencies on other features or external work]
   
   ## Testing Strategy
   - [How this feature will be tested]
   
   ## Documentation Updates
   - [What docs need updating when this is complete]
   
   ## Acceptance Criteria
   - [ ] Criterion 1
   - [ ] Criterion 2
   ```

3. **Create initial task files**
   - Use template from `backlog-add-task` skill
   - Start with mol-<abc>-001.md

4. **Verify structure**
   - Check that index.md renders correctly
   - Verify all task links work
   - Ensure feature acronym is exactly 3 letters

## Task File Template
See `backlog-add-task` skill for the standard task file format.

## Example
Creating OpenTelemetry feature with acronym "ote":

```
.backlog/opentelemetry/
├── index.md
├── mol-ote-001.md
├── mol-ote-002.md
└── mol-ote-003.md
```
