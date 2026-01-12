# Feature: Chlorine PR creation

## Goal

Create a PR from the task branch/worktree.

## Current state

- Chlorine is a stub worker (plus optional `.molecular/chlorine.sh` hook support).

## Detailed implementation steps

1. Decide branch naming and base branch
   - For each task, create a branch in the worktree, e.g.:
     - `molecular/<task_id>`
   - Decide base branch discovery:
     - default to current branch at task creation
     - or config key `base_branch`

2. Ensure worktree has a branch
   - If worktree is detached, create/switch to task branch.

3. Ensure changes are committed
   - Decide policy:
     - either Carbon commits, or Chlorine commits.
   - v1 simplest: Chlorine runs:
     - `git status --porcelain` to see changes
     - `git add -A && git commit -m "molecular: <task_id>"`

4. Create PR with `gh`
   - Use `gh pr create` with:
     - title derived from prompt or task id
     - body includes summary + reproduction steps
   - Capture stdout/stderr into attempt log.

5. Persist PR URL
   - Save PR URL into `result.json` and DB (task record field if we add one).

6. Error handling
   - If `gh` missing:
     - mark task as finished-without-pr or failed (decide policy)
     - record actionable message.

7. Tests
   - Use a fake Runner / command wrapper to simulate `gh` output.
   - Verify attempt artifacts contain the PR URL.

## Acceptance criteria

- Successful end-to-end run produces PR URL in artifacts.
- If `gh` missing, the failure is explicit and actionable.
