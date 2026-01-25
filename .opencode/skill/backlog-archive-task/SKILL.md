---
name: backlog-archive-task
description: Mark a .backlog task DONE and move it to .backlog/.archive/
license: MIT
compatibility: opencode
---

## Purpose
Archive a backlog task by:
1) updating its header `Status` to `DONE` with a date, and
2) moving the task file from `.backlog/` to `.backlog/.archive/`.

## Prerequisites
- The task file exists in `.backlog/` and matches `TXXX-*.md`.

## Procedure
Use `todowrite` with the below checklist.

1. **Identify the task**
   - Ask the human for the task ID (`TXXX`) or the exact filename.
   - Locate the file in `.backlog/` (NOT in `.backlog/.archive/`).

2. **Update task header**
   - Ensure the task has a header section immediately below the `# ...` title containing `Status:`.
   - Set it to the exact format:
     - `**Status:** DONE (YYYY-MM-DD)`
   - If the task previously had `**Status:** TODO`, replace it.
   - If the task has no `Status` header, insert one.

3. **Move to archive**
   - Ensure `.backlog/.archive/` exists (create it if missing).
   - Move the file to `.backlog/.archive/<same-filename>`.

4. **Verify**
   - The task no longer exists in `.backlog/`.
   - The archived file exists in `.backlog/.archive/`.
   - The archived file’s `Status` is `DONE (YYYY-MM-DD)`.

## Notes
- Archiving is intentionally simple: no registry/index updates.
- Do not change the task ID or filename during archive (only status + location).
