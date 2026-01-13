# Feature: Helium real execution

## Goal

Make Helium actually review the Carbon output and drive the review loop based on structured results.

## Current state

- Helium is stubbed and uses prompt markers (`needs-changes`, etc.) in tests.

## Detailed implementation steps

1. Define inspector execution interface
   - Reuse the same `Runner` abstraction as Carbon.
   - Define an inspector command in config.

2. Define the review contract
   - Define a machine-readable output format produced by Helium, e.g. JSON in stdout:

```json
{ "decision": "approved", "summary": "...", "action_items": [] }
```

   - Decisions:
     - `approved`
     - `changes_requested`
     - `rejected`

3. Implement parsing and persistence
   - Parse Helium output.
   - Persist decision + summary:
     - in `attempts` table fields (or extend schema)
     - in `result.json`

4. Drive state transitions
   - If approved => advance to Chlorine.
   - If changes_requested => increment review retry counter and send back to Carbon.
   - If rejected => mark task failed.

5. Retry semantics
   - Separate transient execution failures (process exit non-zero, timeouts) from “changes requested”.
   - Ensure budgets are enforced and surfaced.

6. Tests
   - Fake Runner returns JSON outputs for each decision.
   - Ensure loop: Carbon -> Helium(changes) -> Carbon -> Helium(approved).

## Acceptance criteria

- Helium can request changes and the task loops back to Carbon.
- Helium can approve and advance to Chlorine.
- All decisions are visible in artifacts and status.
