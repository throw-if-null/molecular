import json
import os
import subprocess
import sys
from typing import Any, Dict, List


def validate_builder_result(data: Any) -> Dict[str, Any]:
    errors: List[Dict[str, str]] = []

    if not isinstance(data, dict):
        errors.append(
            {
                "path": "<root>",
                "code": "type_error",
                "message": "builder_result must be a JSON object",
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

        summary = work.get("summary")
        if not isinstance(summary, str) or not summary.strip():
            errors.append(
                {
                    "path": "work.summary",
                    "code": "required",
                    "message": "work.summary must be a non-empty string",
                }
            )

        complexity = work.get("complexity")
        if complexity not in ("low", "medium", "high"):
            errors.append(
                {
                    "path": "work.complexity",
                    "code": "invalid_enum",
                    "message": "work.complexity must be one of 'low', 'medium', 'high'",
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
    result_path = os.path.join(worktree, "builder_result.json")

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
                "path": "builder_result.json",
                "code": "file_missing",
                "message": "builder_result.json not found in worktree root",
            }
        )
    except json.JSONDecodeError as e:
        status = "invalid_json"
        errors.append(
            {
                "path": "builder_result.json",
                "code": "invalid_json",
                "message": f"builder_result.json is not valid JSON: {e}",
            }
        )
    else:
        validation = validate_builder_result(input_data)
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
