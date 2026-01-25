---
name: backlog-start-task
description: Mark a .backlog task IN PROGRESS, set started_on, and create a working checklist.
license: MIT
compatibility: opencode
---

## Purpose
Start work on an existing backlog task by:
1) updating the standardized header table (`status`, `started_on`), and
2) creating a `todowrite` checklist for execution based on the task’s subtasks.

## Prerequisites
- The task file exists in `.backlog/` and matches `TXXX-*.md`.

## Procedure
Use `todowrite` with the below checklist.

1. **Identify the task**
   - Ask the human for the task ID (`TXXX`) or the exact filename.
   - Locate the file in `.backlog/` (NOT in `.backlog/.archive/`).

2. **Update task header table**
   - Update the standardized header table (immediately below the title):
     - set `status` to `IN PROGRESS`
     - set `started_on` to `YYYY-MM-DD HH:mm`
   - If the header table is missing, add it.
   - If `started_on` is already set, ask the human whether to keep it or overwrite it.

3. **Create the working checklist (todowrite)**
   - Parse the task file for an optional `## Subtasks` section.
   - If subtasks exist:
     - create one todo per subtask (use the subtask ID like `TXXX-01` in the todo content).
   - If no subtasks exist:
     - create a single todo for the main task (`TXXX`).
   - Set exactly one item to `in_progress` (typically the first actionable item).
   - All other items should be `pending`.

4. **Verify**
   - The task file’s header table has:
     - `status` = `IN PROGRESS`
     - `started_on` = `YYYY-MM-DD HH:mm`
   - The `todowrite` list mirrors the subtasks (or the main task if none).

## Notes
- This skill does not move files; archiving is handled by `backlog-archive-task`.
