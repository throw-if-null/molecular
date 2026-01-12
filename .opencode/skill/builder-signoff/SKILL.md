---
name: builder-signoff
description: Mandatory sign-off procedure for Builder (public output + builder_result.json + validation).
---

## Purpose
Execute Builder sign-off deterministically so Foreman can validate and proceed.

This procedure is **MANDATORY** and must be executed as the final step of every Builder run — even when blocked or when earlier steps fail.

## Required behavior
- Always produce a **Run Summary** message with the required sections.
- Always write `builder_result.json` to the worktree root.
- Always run `validate_builder_result` and fix any issues it reports.
- If you are blocked or cannot complete build/test steps, still complete this procedure with your best available information.

## Run Summary (MANDATORY format)
Your final chat output MUST include these sections:
1. `Summary`
   - 1–3 short bullets describing the change and why.
2. `Checks run + results`
   - Commands you ran (e.g. `pnpm lint`, `pnpm check`, `pnpm test:unit`, `pnpm prepack`) and pass/fail, or why skipped.
3. `Notes / Risks`
   - Important caveats, known failures, follow-ups, or tradeoffs.
4. `Public API & A11y`
   - Briefly state any public API or accessibility impact.
5. `Checklist (TODO snapshot)`
   - Print the current `todowrite` TODO list (all items + final statuses).

## Foreman Handoff: `builder_result.json` (MANDATORY)
You MUST write a JSON file named `builder_result.json` in the repository/worktree root.

The file MUST contain EXACTLY one JSON object matching this schema:
```json
{
  "run": {
    "status": "ok" | "failed",
    "failed_step": "..." | null,
    "error": "..." | null
  },
  "work": {
    "summary": "...",
    "complexity": "low" | "medium" | "high"
  } | null
}
```

Rules:
- If `run.status` is `ok`, `work` MUST be an object.
- If `run.status` is `failed`, `work` MUST be `null`.

After writing `builder_result.json`, you MUST run `validate_builder_result` and fix any reported issues until it passes.

If `validate_builder_result` cannot be executed (tool missing/unavailable), treat it as a hard failure:
- set `run.status = "failed"`
- set `run.failed_step = "validate_builder_result"`
- set `run.error` to the exact error output
- set `work = null`
Then still write `builder_result.json` and include the failure in the Run Summary.
