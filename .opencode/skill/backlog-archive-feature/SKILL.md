---
name: backlog-archive-feature
description: Archive a completed feature to .backlog/.archive/
---

## Purpose
Move a completed feature to the archive with proper date prefix and completion summary.

## Prerequisites
- All tasks in feature are `done` or `cancelled`
- Feature is tested and merged
- No open blockers or follow-up work

## Procedure

1. **Verify completion**
   - Check index.md task table - all tasks should be `done` or `cancelled`
   - Verify acceptance criteria are met
   - Ensure feature is merged to main branch
   - Confirm no follow-up tasks remain

2. **Update index.md with completion summary**
   - Add this section at the top of index.md (after title):
     ```markdown
     ## ✅ Completed - YYYY-MM-DD
     
     **Summary:** [Brief summary of what was delivered]
     
     **Merged in:** PR #NNN or commit SHA
     
     **Key outcomes:**
     - Outcome 1
     - Outcome 2
     
     **Follow-up work:** [Link to new feature/tasks if applicable, or "None"]
     ```

3. **Rename feature folder with date prefix**
   ```bash
   # Format: YYYYMMDD-<feature_name>
   mv .backlog/<feature_name> .backlog/YYYYMMDD-<feature_name>
   ```
   
   Use today's date for archiving.

4. **Move to archive**
   ```bash
   mv .backlog/YYYYMMDD-<feature_name> .backlog/.archive/
   ```

5. **Verify archive structure**
   - Check that folder is in `.backlog/.archive/YYYYMMDD-<feature_name>/`
   - Verify index.md has completion summary
   - Ensure all task files are preserved

## Example

Archiving prompt-engine feature on 2026-01-17:

```bash
# 1. Updated index.md with completion summary

# 2. Rename with date
mv .backlog/prompt-engine .backlog/20260117-prompt-engine

# 3. Move to archive
mv .backlog/20260117-prompt-engine .backlog/.archive/

# Result:
.backlog/.archive/20260117-prompt-engine/
├── index.md           # Has completion summary at top
├── mol-pte-001.md
├── mol-pte-002.md
└── ...
```

## Post-Archive

- Archive is read-only reference (no further edits)
- If follow-up work is needed, create a new feature
- Link back to archived feature from new feature if related

## Notes

- Archive preserves full history including task progression
- Folder sort order (YYYYMMDD prefix) shows chronological completion
- Completion summary in index.md provides at-a-glance context
