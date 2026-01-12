---
name: inspector-signoff
description: Mandatory final handoff procedure for Inspector (public output + inspector_result.json + validation).
---

## Purpose
Execute Inspector sign-off deterministically so Foreman can continue.

This procedure is **MANDATORY** and must be executed as the final step of every Inspector run — even when blocked or when earlier steps fail.

## Required behavior
- Always produce a **Run Summary** message with the required sections.
- Always write `inspector_result.json` to the worktree root.
- Always run `validate_inspector_result` and fix any issues it reports.
- If you are blocked or cannot complete review steps, still complete this procedure with your best available information.

## Run Summary (MANDATORY format)
Your final chat output MUST include these sections:
1. `Summary`
   - 1–3 short bullets summarizing verdict and key findings.
2. `Checks run + results`
   - Commands you ran (e.g. `pnpm lint`, `pnpm check`, `pnpm test:unit`, `pnpm prepack`) and pass/fail, or why skipped.
3. `Key issues`
   - Important problems/risks (blockers/major issues).
4. `Public API & A11y`
   - Whether public API changes and accessibility are acceptable; call out concerns.
5. `Next steps for Builder`
   - High-level follow-up work expected from Builder (aligned with `next_tasks` in the JSON).
6. `Checklist (TODO snapshot)`
   - Print the current `todowrite` TODO list (all items + final statuses).

## Foreman Handoff: `inspector_result.json` (MANDATORY)
You MUST write a JSON file named `inspector_result.json` in the repository/worktree root.

The file MUST contain EXACTLY one JSON object matching this schema:
```json
{
  "run": {
    "status": "ok" | "failed",
    "failed_step": "..." | null,
    "error": "..." | null
  },
  "work": {
    "status": "approved" | "changes_requested",
    "issues": [
      {
        "severity": "blocker" | "major" | "minor",
        "description": "...",
        "paths": ["..."]
      }
    ],
    "next_tasks": ["..."]
  } | null
}
```

Rules:
- If `run.status` is `ok`, `work` MUST be an object.
- If `run.status` is `failed`, `work` MUST be `null`.
- The `run.status` CAN'T be `failed` if tool checks are returning errors i.e. `pnpm lint` that would be an issue that needs to get fixed by the `builder` agent.

After writing `inspector_result.json`, you MUST run `validate_inspector_result` and fix any reported issues until it passes.

If `validate_inspector_result` cannot be executed (tool missing/unavailable), treat it as a hard failure:
- set `run.status = "failed"`
- set `run.failed_step = "validate_inspector_result"`
- set `run.error` to the exact error output
- set `work = null`
Then still write `inspector_result.json` and include the failure in the Run Summary.
