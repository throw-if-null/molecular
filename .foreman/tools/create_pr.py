import argparse
import json
import os
import re
import subprocess
import sys
from typing import Any


def run(cmd: str, cwd: str | None = None) -> tuple[int, str, str]:
    env = os.environ.copy()
    proc = subprocess.run(
        cmd,
        shell=True,
        cwd=cwd,
        capture_output=True,
        text=True,
        env=env,
    )
    return proc.returncode, proc.stdout, proc.stderr


def remove_builder_result(worktree: str) -> None:
    path = os.path.join(worktree, "builder_result.json")
    if os.path.exists(path):
        os.remove(path)
    # Ensure it is not staged
    run("git rm --cached builder_result.json || true", cwd=worktree)


def remove_inspector_artifacts(worktree: str) -> None:
    for filename in ["inspector_result.json", "inspector_diff.patch"]:
        path = os.path.join(worktree, filename)
        if os.path.exists(path):
            os.remove(path)
        run(f"git rm --cached {filename} || true", cwd=worktree)


def remove_session_log_artifacts(worktree: str) -> None:
    # Keep logs on disk, but never include them in commits/PRs.
    # OpenCode writes under `.opencode/session-log/` (also remove legacy `session-log/`).
    run("git rm -r --cached session-log .opencode/session-log || true", cwd=worktree)
    # If they are not tracked, they can still be staged via `git add -A`.
    # Explicitly unstage them.
    run("git restore --staged --worktree -- session-log .opencode/session-log || true", cwd=worktree)


def update_todo(task_id: str, repo_root: str) -> None:
    todo_path = os.path.join(repo_root, "docs", "TODO.md")
    if not os.path.exists(todo_path):
        return

    with open(todo_path, "r", encoding="utf-8") as f:
        content = f.read()

    # Pattern: - [ ] [ds9-3 · 2.2 ...] or similar
    # We mark the first occurrence for this task id as [x]
    pattern = rf"- \[ \] \[{re.escape(task_id)}"
    replacement = f"- [x] [{task_id}"

    new_content, count = re.subn(pattern, replacement, content, count=1)
    if count:
        with open(todo_path, "w", encoding="utf-8") as f:
            f.write(new_content)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--task-id", required=True, help="Task id, e.g. ds9-3")
    parser.add_argument("--title", required=True, help="PR title")
    parser.add_argument("--body", required=True, help="Task body/description for PR")
    parser.add_argument(
        "--worktree", required=True, help="Path to git worktree for this task"
    )
    parser.add_argument(
        "--branch", required=True, help="Local branch name for this task"
    )
    args = parser.parse_args()

    repo_root = os.getcwd()

    # 1) Remove result artifacts from PR
    remove_builder_result(args.worktree)
    remove_inspector_artifacts(args.worktree)
    remove_session_log_artifacts(args.worktree)

    # 2) Update docs/TODO.md marking task as completed inside the worktree
    update_todo(args.task_id, args.worktree)

    # 3) Commit any remaining changes in the worktree (if not already committed)
    run("git add -A", cwd=args.worktree)

    # If there is nothing to commit, git commit will fail; that's fine.
    run("git commit -m 'chore: finalize task' || true", cwd=args.worktree)

    # 4) Push branch and create PR with task body as description
    pr_body = args.body
    quoted_body = json.dumps(pr_body)

    push_cmd = f"git push -u origin {args.branch}"
    ret, out, err = run(push_cmd, cwd=args.worktree)
    if ret != 0:
        print(json.dumps({"ok": False, "step": "push", "stdout": out, "stderr": err}))
        sys.exit(0)

    pr_cmd = (
        "gh pr create "
        f"--title {json.dumps(args.title)} "
        f"--body {quoted_body} "
        f"--head {args.branch} "
        "--draft"
    )
    ret, out, err = run(pr_cmd, cwd=args.worktree)

    pr_url: Any = None
    if ret == 0:
        # gh pr create usually prints the URL on the last line
        lines = [ln.strip() for ln in out.splitlines() if ln.strip()]
        if lines:
            pr_url = lines[-1]

    # 5) Remove worktree and local branch
    run(f"git worktree remove {args.worktree} --force || true", cwd=repo_root)
    run(f"git branch -D {args.branch} || true", cwd=repo_root)

    result = {
        "ok": ret == 0,
        "pr_url": pr_url,
        "stdout": out,
        "stderr": err,
    }
    json.dump(result, sys.stdout)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
