---
name: backlog-update-status
description: Update task status in both task file and index.md
---

## Purpose
Keep task status current as work progresses through the feature implementation.

## Prerequisites
- Task file exists in `.backlog/<feature_name>/mol-<abc>-NNN.md`
- New status is valid: `todo | in_progress | blocked | done | cancelled`

## Procedure

1. **Update task file**
   - Open `.backlog/<feature_name>/mol-<abc>-NNN.md`
   - Find `**Status:** <current_status>`
   - Replace with new status
   - If status is `blocked`, add reason to Notes section
   - If status is `done`, verify all acceptance criteria are checked

2. **Update index.md task table**
   - Open `.backlog/<feature_name>/index.md`
   - Find the row with task ID `mol-<abc>-NNN`
   - Update the Status column
   - Add any relevant notes in the Notes column

3. **Add status change notes (optional but recommended)**
   - In task file Notes section, add timestamp and reason:
     ```markdown
     ## Notes
     
     - 2026-01-17: Status changed to `in_progress` - Starting implementation
     - 2026-01-18: Status changed to `blocked` - Waiting for mol-pte-001 completion
     - 2026-01-19: Status changed to `in_progress` - Dependency resolved
     - 2026-01-20: Status changed to `done` - All tests passing
     ```

## Status Transitions

Valid transitions:
- `todo` → `in_progress`
- `in_progress` → `blocked` (add reason to Notes)
- `in_progress` → `done` (verify acceptance criteria)
- `blocked` → `in_progress` (note what unblocked it)
- `todo` → `cancelled` (explain why in Notes)
- `in_progress` → `cancelled` (explain why in Notes)

## Example

Marking task mol-pte-001 as done:

**Task file change:**
```markdown
**Status:** done  <!-- was: in_progress -->
```

**index.md table change:**
```markdown
| [mol-pte-001](./mol-pte-001.md) | Create templates package | done | Merged in PR #15 |
```

**Task file Notes:**
```markdown
## Notes

- 2026-01-17: Started implementation
- 2026-01-18: Completed, all tests passing, merged in PR #15
```

## Bulk Status Updates

When updating multiple tasks at once (e.g., after a sprint):
1. Update all task files first
2. Then update index.md table in one pass
3. Ensure consistency between task files and index.md
