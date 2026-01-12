import json
import os
import subprocess
import sys
from typing import Any, Dict, List


def validate_inspector_result(data: Any) -> Dict[str, Any]:
    errors: List[Dict[str, str]] = []

    if not isinstance(data, dict):
        errors.append(
            {
                "path": "<root>",
                "code": "type_error",
                "message": "inspector_result must be a JSON object",
            }
        )
        return {"ok": False, "errors": errors}

    run = data.get("run")
    if not isinstance(run, dict):
        errors.append(
            {
                "path": "run",
                "code": "required",
                "message": "run must be an object",
            }
        )
        run_status = None
    else:
        run_status = run.get("status")
        if run_status not in ("ok", "failed"):
            errors.append(
                {
                    "path": "run.status",
                    "code": "invalid_enum",
                    "message": "run.status must be 'ok' or 'failed'",
                }
            )

        failed_step = run.get("failed_step")
        if failed_step is not None and not isinstance(failed_step, str):
            errors.append(
                {
                    "path": "run.failed_step",
                    "code": "type_error",
                    "message": "run.failed_step must be a string or null",
                }
            )

        error = run.get("error")
        if error is not None and not isinstance(error, str):
            errors.append(
                {
                    "path": "run.error",
                    "code": "type_error",
                    "message": "run.error must be a string or null",
                }
            )

    work = data.get("work")
    if run_status == "failed":
        if work is not None:
            errors.append(
                {
                    "path": "work",
                    "code": "invalid",
                    "message": "work must be null when run.status is 'failed'",
                }
            )
        return {"ok": not errors, "errors": errors}

    if run_status == "ok":
        if not isinstance(work, dict):
            errors.append(
                {
                    "path": "work",
                    "code": "required",
                    "message": "work must be an object when run.status is 'ok'",
                }
            )
            return {"ok": not errors, "errors": errors}

        status = work.get("status")
        if status not in ("approved", "changes_requested"):
            errors.append(
                {
                    "path": "work.status",
                    "code": "invalid_enum",
                    "message": "work.status must be 'approved' or 'changes_requested'",
                }
            )

        issues = work.get("issues")
        if not isinstance(issues, list):
            errors.append(
                {
                    "path": "work.issues",
                    "code": "type_error",
                    "message": "work.issues must be an array",
                }
            )
        else:
            if status == "changes_requested" and not issues:
                errors.append(
                    {
                        "path": "work.issues",
                        "code": "required",
                        "message": "work.issues must be non-empty when work.status is 'changes_requested'",
                    }
                )
            for idx, issue in enumerate(issues):
                if not isinstance(issue, dict):
                    errors.append(
                        {
                            "path": f"work.issues[{idx}]",
                            "code": "type_error",
                            "message": "each issue must be an object",
                        }
                    )
                    continue
                severity = issue.get("severity")
                if severity not in ("blocker", "major", "minor"):
                    errors.append(
                        {
                            "path": f"work.issues[{idx}].severity",
                            "code": "invalid_enum",
                            "message": "severity must be 'blocker', 'major', or 'minor'",
                        }
                    )
                desc = issue.get("description")
                if not isinstance(desc, str) or not desc.strip():
                    errors.append(
                        {
                            "path": f"work.issues[{idx}].description",
                            "code": "required",
                            "message": "description must be a non-empty string",
                        }
                    )
                paths = issue.get("paths")
                if not isinstance(paths, list) or not all(
                    isinstance(p, str) and p.strip() for p in paths
                ):
                    errors.append(
                        {
                            "path": f"work.issues[{idx}].paths",
                            "code": "type_error",
                            "message": "paths must be an array of non-empty strings",
                        }
                    )

        next_tasks = work.get("next_tasks")
        if not isinstance(next_tasks, list) or not all(
            isinstance(t, str) and t.strip() for t in next_tasks
        ):
            errors.append(
                {
                    "path": "work.next_tasks",
                    "code": "type_error",
                    "message": "work.next_tasks must be an array of non-empty strings",
                }
            )

    return {"ok": not errors, "errors": errors}


def get_changed_files() -> List[Dict[str, str]]:
    try:
        proc = subprocess.run(
            ["git", "status", "--porcelain"],
            check=False,
            capture_output=True,
            text=True,
        )
        lines = [ln for ln in proc.stdout.splitlines() if ln.strip()]
        files: List[Dict[str, str]] = []
        for line in lines:
            status = line[:2].strip() or line[:2]
            path = line[3:].strip()
            if path:
                files.append({"path": path, "status": status})
        return files
    except Exception:
        return []


def main() -> None:
    worktree = os.getcwd()
    result_path = os.path.join(worktree, "inspector_result.json")

    input_data: Any = None
    errors: List[Dict[str, Any]] = []
    status: str

    try:
        with open(result_path, "r", encoding="utf-8") as f:
            input_data = json.load(f)
    except FileNotFoundError:
        status = "file_missing"
        errors.append(
            {
                "path": "inspector_result.json",
                "code": "file_missing",
                "message": "inspector_result.json not found in worktree root",
            }
        )
    except json.JSONDecodeError as e:
        status = "invalid_json"
        errors.append(
            {
                "path": "inspector_result.json",
                "code": "invalid_json",
                "message": f"inspector_result.json is not valid JSON: {e}",
            }
        )
    else:
        validation = validate_inspector_result(input_data)
        if bool(validation.get("ok")):
            status = "valid"
        else:
            status = "invalid_schema"
            errors.extend(validation.get("errors", []))

    data: Dict[str, Any] = {"run": None, "work": None}
    if status == "valid" and isinstance(input_data, dict):
        run = input_data.get("run")
        work = input_data.get("work")
        if isinstance(run, dict):
            data["run"] = {
                "status": run.get("status"),
                "failed_step": run.get("failed_step"),
                "error": run.get("error"),
            }
        if isinstance(work, dict) or work is None:
            data["work"] = work

    output = {
        "status": status,
        "errors": errors,
        "data": data,
        "changed_files": get_changed_files(),
    }

    json.dump(output, sys.stdout)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
