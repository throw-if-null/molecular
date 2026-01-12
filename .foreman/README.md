# Foreman (OpenCode Orchestration)

Foreman coordinates parallel OpenCode **Builder** agents, gates changes through an **Inspector**, and opens GitHub pull requests. It isolates concurrent work with **git worktrees**, enforces retry limits (replace stuck Builders), and notifies developers via **Slack** when human input is required.

This repository provides two layers:

* **OpenCode-first UX**: you start from OpenCode by selecting the **Foreman** agent and pasting a prompt.
* **Python supervisor**: `foreman.py` is the reliable control/data plane that owns worktrees, server processes, retries, PR creation, and persisted state.

---

## Roles

### Foreman (supervisor)

Owns and enforces:

* task queue and task state (persisted via **SQLite** or similar for crash recovery)
* git worktrees/branches lifecycle
* OpenCode server process lifecycle
* retry limits and “stuck” handling
* diff/patch collection and Inspector review routing
* GitHub PR creation
* Slack notifications for human input

### Builder (implementer)

Works inside an isolated worktree (ideally containerized):

* edits code, runs tests, commits changes
* signals completion with `READY_FOR_REVIEW` + JSON
* signals blocking questions with `NEEDS_HUMAN_INPUT` + JSON

**Note:** For true isolation (dependencies, runtime environment), it is recommended to run each Builder in a lightweight container (Docker/Podman) that mounts the specific worktree. This prevents environment pollution between concurrent agents.

### Inspector (reviewer)

Review-only gatekeeper:

* reviews patch/commit
* outputs **only** a JSON decision: `{ status, issues, next_tasks }`
* no edit/bash permissions

**Optimization:** To save AI tokens and time, Foreman runs automated "pre-flight" checks (lint, build, unit tests) *before* invoking the Inspector. If the code fails to compile or lint, it is rejected immediately without an expensive LLM review.

---

## High-level flow

For each task:

1. Foreman creates a git worktree + unique branch.
2. Foreman starts a dedicated **Builder OpenCode server** rooted at the worktree.
3. Builder implements and commits.
4. Foreman produces a patch from the commit.
5. **(Optional)** Foreman runs pre-flight checks (lint, test). If these fail, the task is returned to the Builder immediately.
6. Inspector reviews patch and returns a JSON decision.
7. Foreman either:

   * opens a GitHub PR (default **draft**) on approval, or
   * feeds issues back to Builder and loops.
7. If Builder is stuck (default 3 review cycles), Foreman stops the Builder, deletes the worktree, and retries with a fresh Builder.
8. If human input is needed, Foreman notifies via Slack and moves the task to `WAITING_FOR_HUMAN` until answered in Foreman’s OpenCode session.

---

## Requirements

### System

* Python 3.11+
* `git`
* OpenCode CLI (`opencode`) on PATH
* GitHub CLI (`gh`) authenticated **or** a GitHub token (REST)
* Slack bot token (for notifications)

### Python packages

* `httpx`
* `slack_sdk`

Install:

```bash
pip install -r requirements.txt
# or
pip install httpx slack_sdk
```

---

## Configuration

### `foreman.json`

```json
{
  "instance": {
    "name": "dev-A",
    "state_dir": ".foreman/dev-A",
    "worktrees_dir": ".oc_worktrees/dev-A"
  },
  "github": {
    "repo": "OWNER/REPO",
    "base_branch": "main",
    "draft_by_default": true,
    "use_gh_cli": true,
    "default_labels": ["autocode"],
    "default_reviewers": [],
    "default_assignees": []
  },
  "slack": {
    "enabled": true,
    "notify_by": "email",
    "message_prefix": "[Foreman]"
  },
  "limits": {
    "max_review_cycles": 3,
    "inspector_max_inflight_reviews": 4
  }
}
```

Notes:

* `instance.name` is included in Slack notifications so it’s easy to find the right Foreman when multiple instances run.
* `state_dir` and `worktrees_dir` must be unique per instance to avoid collisions.
* PRs are created as **draft** by default; override per task or set `draft_by_default=false`.

### Environment variables

GitHub:

* If using `gh`: authenticate once with `gh auth login`.
* If using REST: `GITHUB_TOKEN`.

Slack:

* `SLACK_BOT_TOKEN`

Slack app scopes typically required:

* `users:read.email`
* `chat:write`
* `im:write`

---

## Tasks

### `tasks.json`

```json
[
  {
    "id": "feat-123",
    "title": "Add pagination to widgets endpoint",
    "instructions": "Implement limit/offset for GET /widgets. Update docs and tests.",
    "base_ref": "main",
    "assignee_email": "dev@company.com",
    "test_command": "pytest -q",
    "draft": true
  }
]
```

Per-task overrides:

* `draft` (overrides config default)
* `labels`, `reviewers`, `assignees`
* `pr_title`, `pr_body`

---

## OpenCode-first usage

1. Open the repository in OpenCode.
2. Select the **Foreman** agent.
3. Paste a prompt describing the work.

Example prompt:

> Implement pagination for GET /widgets (limit/offset), update docs and tests. Assignee [dev@company.com](mailto:dev@company.com). Run pytest -q. Create a PR.

Foreman will:

* materialize `tasks.json` (or an OpenSpec change folder, if configured)
* run the supervisor loop
* report status and next actions

---

## CLI usage

### Run

```bash
python foreman.py run --repo /path/to/repo --config foreman.json --tasks tasks.json --concurrency 4
```

### Status

```bash
python foreman.py status --config foreman.json
```

### Answer a human-input request

When a task is `WAITING_FOR_HUMAN`, answer in the Foreman OpenCode session or via CLI:

```bash
python foreman.py answer --config foreman.json --task feat-123 --answer "yes"
```

---

## Contracts

### Builder completion

Builder must end its final message with:

1. Marker line:

```
READY_FOR_REVIEW
```

2. JSON object:

```json
{ "commit": "<sha>", "summary": "...", "tests_run": ["..."], "notes": "..." }
```

### Builder human-input request

1. Marker line:

```
NEEDS_HUMAN_INPUT
```

2. JSON object:

```json
{
  "question": "...",
  "options": ["..."],
  "blocking": true,
  "context": ["..."]
}
```

### Inspector decision

Inspector must output **only** one JSON object:

```json
{
  "status": "approved|changes_requested",
  "issues": [
    { "severity": "blocker|major|minor", "description": "...", "paths": ["..."] }
  ],
  "next_tasks": ["..."]
}
```

---

## Stuck handling (Builder replacement & Refinement)

Default rule:

* `max_review_cycles = 3`

Behavior:

* After 3 `changes_requested` cycles, Foreman marks the task stuck.
* **Refinement (Recommended):** Instead of immediately killing the Builder, Foreman triggers a `NEEDS_HUMAN_INPUT` event. The developer is asked if they want to refine the task/prompt, as 3 failures often indicate ambiguous requirements rather than a "bad" agent.
* If no refinement is offered or the cycle limit is hit again, Foreman stops the Builder, deletes the worktree, and retries with a fresh Builder.

## Merge Conflicts

When multiple Builders run in parallel on the same repo, merge conflicts are possible.
* **Mitigation:** Structure tasks to be modular and orthogonal (e.g., "Agent A owns Component X, Agent B owns Component Y").
* **Resolution:** Foreman does not ask agents to resolve complex merge conflicts. If a conflict occurs during the PR phase or sync, the task is flagged for human intervention.

---

## Concurrency and isolation

* Each Builder runs in its own git worktree and branch.
* Each Builder gets its own OpenCode server process rooted in that worktree.
* Inspector runs as a shared review service, but Foreman keeps one Inspector session per task to avoid cross-task context bleed.

---

## GitHub PR creation

On approval, Foreman:

1. pushes the task branch
2. creates a PR

Default: draft PR (`draft_by_default=true`).

PR body includes:

* task summary
* tests run
* Inspector decision metadata

---

## Troubleshooting

### PR creation fails

* If using `gh`, ensure you are authenticated: `gh auth status`.
* If using REST, ensure `GITHUB_TOKEN` has repo permissions.

### Slack notifications fail

* Verify `SLACK_BOT_TOKEN` and Slack app scopes.
* Verify the task has `assignee_email` and that the email matches the Slack user profile.

### Worktree cleanup issues

* Foreman uses forced removal for stuck tasks; if a worktree directory remains, ensure no process is still holding files open.

---

## License

Add your license here.
