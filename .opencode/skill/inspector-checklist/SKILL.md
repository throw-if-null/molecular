---
name: inspector-checklist
description: Inspector review checklist used to drive todowrite and review order.
---

## Purpose
Use this checklist to create and maintain a `todowrite` TODO list and to execute Inspector review in a predictable, traceable order.

## Required behavior
- Immediately create a TODO list with `todowrite` that mirrors the checklist items.
- Keep the TODO list updated as you work (mark items `in_progress` / `completed` / `cancelled`).
- Keep exactly one item `in_progress` at a time.
- The TODO list is for determinism/traceability only; it does NOT replace required on-disk artifacts.

## Review checklist (follow in order when possible)
- [ ] (MANDATORY) Initialize the todo list with `todowrite` (mirror this checklist)
- [ ] (MANDATORY) Read `AGENTS.md` and `REVIEW_RULEBOOK.md` to refresh requirements and constraints
- [ ] (MANDATORY) Read and understand `builder_result.json` (summary + complexity)
- [ ] (MANDATORY) Examine the diff (`inspector_diff.patch` or `git diff`) and the updated code
- [ ] (MANDATORY) Run `pnpm install` (`components/`) if dependencies are missing
  - If `pnpm install` cannot run (for example due to network restrictions), treat it as a hard failure:
    - write `inspector_result.json` with `run.status = "failed"`, `run.failed_step = "pnpm install"`, `run.error` set to the exact error output, and `work = null`
    - then proceed to the `inspector-signoff` skill so Foreman can stop safely.
- [ ] (MANDATORY) Run `pnpm lint` (`components/`) and record pass/fail
  - If it fails due to formatting (Prettier), you MAY run `pnpm format` (`components/`) and then re-run `pnpm lint`.
- [ ] (MANDATORY) Run `pnpm check` (`components/`) and record pass/fail
- [ ] (MANDATORY) Run `pnpm test:unit` (or broader `pnpm test` when appropriate) (`components/`) and record pass/fail
- [ ] (OPTIONAL) Run `pnpm prepack` (`components/`) when packaging changes are involved and record pass/fail
  - If you are unsure whether packaging is involved, run it.
- [ ] (MANDATORY) Review the changes against `AGENTS.md` and `REVIEW_RULEBOOK.md`:
  correctness, accessibility, Svelte 5/runes rules, public API stability, tests, and docs
- [ ] (MANDATORY) Decide whether the change is acceptable or if changes are required
- [ ] (CRITICAL) (MANDATORY) Execute the `inspector-signoff` skill (writes `inspector_result.json` + validates + prints Public Handoff)
