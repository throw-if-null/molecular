---
name: backlog-do-pr
description: Create PR + wait for human review + post-merge advance backlog checklist.
license: MIT
compatibility: opencode
---

## Purpose
Drive a deterministic loop for: prepare branch → test → commit → push → open PR → wait for human review → after merge, advance the backlog checklist (`todowrite`) and sync local `main`.

## Required behavior (determinism)
- Immediately create and maintain a `todowrite` checklist that mirrors this skill’s steps.
- Keep the checklist updated as you work (mark items `in_progress` / `completed` / `cancelled`).
- Keep exactly one item `in_progress` at a time.
- If a `todowrite` list already exists (e.g. from `backlog-start-task`):
  - first read it (`todoread`) and preserve its contents so you can restore it after PR merge.

## Safety rules (must follow)
- No destructive git actions (no reset --hard, no rebase -i, no force push).
- Do not push to `main`.
- Only commit/push/create a PR when the user invoked this skill explicitly to do so.
- PRs must be created using `gh pr create` (use a heredoc for the body).

## Step-by-step behavior

### 0) Initialize the `todowrite` checklist
Create a `todowrite` list with these items (in this order):
1. Snapshot existing `todowrite` (if any)
2. Ensure working tree ready + on feature branch
3. Run tests (`go test ./...`)
4. Stage changes + commit
5. Push branch
6. Create PR (`gh pr create`)
7. Ask human for review and wait
8. After merge: restore + advance backlog checklist
9. Sync local `main` (`git checkout main`, `git pull --ff-only`)

Then proceed, keeping exactly one item `in_progress`.

### 1) Snapshot existing `todowrite` (if any)
1. Run `todoread`.
2. If a list exists, preserve it so you can restore it later.
   - Goal: after PR merge, you will restore that list, then mark the current `in_progress` item `completed` and advance the next `pending` item to `in_progress`.

### 2) Ensure working tree is ready and on a feature branch
1. Inspect state:
   - `git status`
   - `git diff` (unstaged)
   - `git diff --staged` (staged)
2. If there are **no changes** (status clean and no untracked files), **stop**: do not create a commit or PR.
3. Confirm current branch:
   - `git branch --show-current`
4. If on `main`:
   - create and switch to a feature branch: `git switch -c feature/<short-name>`
   - keep name short and descriptive (e.g. `feature/config-load-fix`).

### 3) Run Go tests and record results
1. Run at minimum:
   - `go test ./...`
2. Capture:
   - pass/fail
   - the exact command(s) run
   - brief failure summary if failing
3. These results must be included in the PR body under **Testing**.

### 4) Stage, commit, and push (safe)
1. Stage relevant changes:
   - use `git add -A` (or target paths) after reviewing what will be included.
2. Verify staged diff:
   - `git diff --staged`
3. Create **one** local commit:
   - commit message should follow repo style: concise, imperative, explains “why”.
4. Push the feature branch:
   - If upstream not set: `git push -u origin HEAD`
   - Otherwise: `git push`

### 5) Create PR with `gh pr create`
Create the PR using `gh pr create` and a heredoc body. Include:

```bash
gh pr create --title "<title>" --body "$(cat <<'EOF'
## Summary
- <1–3 bullets describing intent/why>

## Testing
- `go test ./...` (<pass|fail>)
  - Notes: <brief output summary or failure reason>

## Notes / Risks
- <edge cases, rollout considerations, follow-ups>
EOF
)"
```

After creation, return the PR URL.

### 6) Ask for human review and explicitly wait
1. Ask the human to review the PR.
2. **Stop and wait** for feedback (do not continue with more git operations).
3. If human requests changes:
   - implement fixes
   - rerun relevant tests (at minimum `go test ./...`)
   - commit additional changes on the same branch
   - push updates
   - comment/describe updates as requested
   - then wait again.

### 7) After human confirms the PR is merged: update `todowrite` and sync main
Only after the human explicitly confirms the PR is merged:
1. Update `todowrite`:
   - restore the previous backlog checklist (if you replaced it)
   - mark the current `in_progress` backlog item as `completed`
   - set the next `pending` backlog item (if any) to `in_progress`
   - keep exactly one item `in_progress`.
2. Sync local `main` safely:
   - `git checkout main`
   - `git pull --ff-only`

## Failure handling
- **Tests fail**:
  - You may still create the PR **only if the user wants to proceed**.
  - Clearly mark failures in **Testing** and call out risk/impact in **Notes / Risks**.
  - Ask the human whether to proceed before pushing/creating the PR if failures are unexpected.
- **No changes to commit**:
  - Do not create a commit, do not push, do not create a PR.
