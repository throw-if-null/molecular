---
name: backlog-archive-task
description: Mark a .backlog task DONE/CANCELLED and move it to .backlog/.archive/
license: MIT
compatibility: opencode
---

## Purpose
Archive a backlog task by:
1) updating its header table fields (`status`, `finished_on`), and
2) moving the task file from `.backlog/` to `.backlog/.archive/`.

## Prerequisites
- The task file exists in `.backlog/` and matches `TXXX-*.md`.

## Procedure
Use `todowrite` with the below checklist.

1. **Identify the task**
   - Ask the human for the task ID (`TXXX`) or the exact filename.
   - Locate the file in `.backlog/` (NOT in `.backlog/.archive/`).

2. **Update task header**
   - Ask the human for the final status:
     - `DONE` or `CANCELLED`
   - Update the standardized header table (immediately below the title):
     - set `status` to `DONE` or `CANCELLED`
     - set `finished_on` to `YYYY-MM-DD HH:mm`
   - If the header table is missing, add it.

3. **Move to archive**
   - Ensure `.backlog/.archive/` exists (create it if missing).
   - Move the file to `.backlog/.archive/<same-filename>`.

4. **Verify**
   - The task no longer exists in `.backlog/`.
   - The archived file exists in `.backlog/.archive/`.
   - The archived file’s header table has:
     - `status` set to `DONE` or `CANCELLED`
     - `finished_on` set to `YYYY-MM-DD HH:mm`

## Notes
- Archiving is intentionally simple: no registry/index updates.
- Do not change the task ID or filename during archive (only status + location).
