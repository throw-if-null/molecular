# Feature: Better status UX

## Goal

Make `molecular status` and related commands more informative.

## Detailed implementation steps

1. Decide the output contract
   - Human output should include:
     - task id
     - phase
     - status
     - latest attempt id + role + status
     - retry counters/budgets (optional)

2. Server response improvements
   - If current API lacks latest attempt summary, add it to task response JSON.
   - Alternatively, CLI can call an attempts endpoint (if we add one).

3. CLI formatting
   - Keep default output concise.
   - Consider `--json` mode for scripting.

4. Tests
   - CLI tests for formatting.
   - Server tests if API shape changes.

## Acceptance criteria

- A user can diagnose state from `molecular status` without opening the DB.
