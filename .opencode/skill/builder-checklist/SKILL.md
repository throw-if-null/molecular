---
name: builder-checklist
description: Builder implementation checklist used to drive todowrite and execution order.
---

## Purpose
Use this checklist to create and maintain a `todowrite` TODO list and to execute Builder work in a predictable, traceable order.

## Required behavior
- Immediately create a TODO list with `todowrite` that mirrors the checklist items.
- Keep the TODO list updated as you work (mark items `in_progress` / `completed` / `cancelled`).
- Keep exactly one item `in_progress` at a time.
- The TODO list is for determinism/traceability only; it does NOT replace required on-disk artifacts.

## Implementation checklist (follow in order when possible)
- [ ] (MANDATORY) Initialize the todo list with `todowrite` (mirror this checklist)
- [ ] (MANDATORY) Read `AGENTS.md` and, if present, `REVIEW_RULEBOOK.md` to refresh requirements and constraints
- [ ] (MANDATORY) Read the assigned task and restate it briefly
- [ ] (MANDATORY) Identify the files and modules likely involved in the change
- [ ] (MANDATORY) Implement the required changes with minimal, focused diffs
- [ ] (MANDATORY) Update or add tests for any new or changed behavior (Vitest/Playwright as appropriate)
- [ ] (MANDATORY) Run `pnpm install` (`components/`) if dependencies are missing
  - If `pnpm install` cannot run (for example due to network restrictions), treat it as a hard failure:
    - write `builder_result.json` with `run.status = "failed"`, `run.failed_step = "pnpm install"`, `run.error` set to the exact error output, and `work = null`
    - then proceed to the `builder-signoff` skill so Foreman can stop safely.
- [ ] (MANDATORY) Run `pnpm lint` (`components/`) and record pass/fail
  - If it fails due to formatting (Prettier), you MAY run `pnpm format` (`components/`) and then re-run `pnpm lint`.
  - If lint still fails, still proceed; record the failure in your handoff.
- [ ] (MANDATORY) Run `pnpm check` (`components/`) and record pass/fail
  - If it fails, still proceed; record the failure in your handoff.
- [ ] (MANDATORY) Run `pnpm test:unit` (or broader `pnpm test` when appropriate) (`components/`) and record pass/fail
  - If it fails, still proceed; record the failure in your handoff.
- [ ] (OPTIONAL) Run `pnpm prepack` (`components/`) when packaging changes are involved and record pass/fail
  - If you are unsure whether packaging is involved, run it.
- [ ] (MANDATORY) Prepare the Git state: stage all relevant files with `git add` so that `git diff main...HEAD` reflects the full change
- [ ] (MANDATORY) Create a local commit with a clear, concise message
  - If `git commit` fails for any reason, treat it as a hard failure (Foreman depends on the commit for `git diff HEAD...main`):
    - write `builder_result.json` with `run.status = "failed"`, `run.failed_step = "git commit"`, `run.error` set to the exact error output, and `work = null`
    - then proceed to the `builder-signoff` skill so Foreman can stop safely.
- [ ] (CRITICAL) (MANDATORY) Execute the `builder-signoff` skill (writes `builder_result.json` + validates + prints Public Handoff)

## Result JSON requirement (always)
No matter what happens ALWAYS write `builder_result.json` and validate it with `validate_builder_result` (in-process tool).
- If the validator reports issues: fix the JSON and re-run until it passes.
- If the validator tool itself cannot be executed (tool missing/unavailable): treat it as a hard failure and write `builder_result.json` with `run.status = "failed"`, `run.failed_step = "validate_builder_result"`, `run.error` set to the exact error output, and `work = null`, then proceed to the `builder-signoff` skill so Foreman can stop safely.
