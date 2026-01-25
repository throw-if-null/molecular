---
name: backlog-add-task
description: Create a new backlog task file in .backlog/
license: MIT
compatibility: opencode
---

## Purpose
Create a new task in `.backlog/` (flat layout) following the existing “T000/T001” style.

This skill is interactive: create the task file early, then iterate with the human until the task is ready to be picked up for development.

## When to use me
- Use when the human says: “Create a task …” or provides a new `TXXX-...` task name.
- Do NOT use for archiving; use `backlog-archive-task` for that.

## Prerequisites
- The human can provide an explicit task ID (`TXXX`) and a short name.
  - If the ID is not provided, you MUST compute and propose the next available `TXXX`.

## Procedure
Use `todowrite` with the below checklist.

1. **Determine the task identity**
   - Ask for (or derive) the `TXXX` ID.
     - Scan `.backlog/` for files matching `T\d\d\d-*.md` (excluding `.backlog/.archive/`).
     - Next ID = (max existing numeric ID) + 1.
     - IDs are zero-padded and start at `T000`.
   - Ask for a short name to use in the filename.
   - Filename format:
     - `TXXX-My_task_name.md` (underscores allowed; keep it readable).

2. **Create the task file immediately (skeleton first)**
   - Create `.backlog/<filename>` with the template below.
   - Set `**Created:**` to today’s date (`YYYY-MM-DD`).
   - Leave sections as placeholders; we will fill them in during iteration.

3. **Iterate with the human to refine scope**
   - Fill in:
     - Goal
     - Context
     - Non-goals
     - Deliverables
     - Testing
     - Acceptance criteria
   - Ask for constraints and bake them into the task:
     - dependency policy (new deps disallowed unless explicitly approved)
     - target packages/areas
     - must-have tests
     - preferred branch name

4. **Break down into subtasks (in the SAME file) when helpful**
   - If the work is multi-phase or spans multiple areas, add a `## Subtasks` section.
   - Subtask IDs must be:
     - `TXXX-YY` (e.g. `T001-01`, `T001-02`)
   - Subtasks live inside the task file (no separate files).
   - Each subtask should have:
     - a short title
     - a brief definition of done
     - optional owners/agents (if you want)
     - optional dependencies between subtasks
   - Prefer writing subtasks as a checklist so it’s easy to track progress.

5. **Ensure the task is “ready for development”**
   - The task should be implementable without guessing:
     - acceptance criteria are testable
     - testing instructions are explicit
     - subtasks (if any) cover the deliverables
     - scope boundaries are clear (non-goals)
   - Do one last human confirmation: “Is this ready to pick up?”

6. **Task file template**

   ```markdown
   # TXXX - <Short descriptive title>

   **Status:** TODO
   **Created:** YYYY-MM-DD

   ## Goal
   [Clear, concise statement of what this task accomplishes]

   ## Context
   [Why this task is needed; links to related tasks/PRs/issues if any]

   ## Non-goals
   - [What is explicitly out of scope]

   ## Design / Approach
   - [Key decisions, constraints, trade-offs]
   - [Any compatibility/backwards-compat notes]

   ## Deliverables
   - [Concrete outputs]

   ## Implementation notes

   ### Files to create/modify
   - `path/to/file.go` — …
   - `path/to/file_test.go` — …

   ### Steps
   1. …
   2. …

   ## Testing
   - [ ] `go test ./...`
   - [ ] Targeted tests: …
   - [ ] Manual verification: …

   ## Acceptance criteria
   - [ ] …
   - [ ] All tests passing

   ## Notes
   [Edge cases, follow-ups]

   ## Subtasks (optional)
   - [ ] **TXXX-01** — <subtask title>
     - DoD: <definition of done>
   - [ ] **TXXX-02** — <subtask title>
     - DoD: <definition of done>
   ```

7. **Verify**
   - Task file renders correctly
   - Filename matches `TXXX-*.md`
   - The `Status` header is present and set to `TODO`

## Example Task Addition

If the next available ID is `T002` and the short name is `Implement_some_task`:

1. Create `.backlog/T002-Implement_some_task.md`
2. Fill in the template above
3. Add subtasks like `T002-01`, `T002-02` if needed
4. Iterate with the human until scope + acceptance criteria are crisp
