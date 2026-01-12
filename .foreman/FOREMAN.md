# Foreman (The name will change into something gender neutral later)

A Foreman is an orchestration and agent‑tooling bundle that provides a reusable template for running Builder + Inspector agent workflows across consumer repositories. The assets under `.foreman/` are intended to be vendored into consumer repos (like this one) and can later be extracted into their own repository for distribution.

This document describes the generic OpenCode tool contracts, example wiring to the provided Python implementations, suggested prompt snippets for agents, and next steps. It intentionally avoids any repo‑specific details so consumers can adapt Foreman to their projects.

---

## Goals

- Provide generic, repo‑agnostic tools and skills that agents can call to validate and reason about Builder/Inspector handoffs.
- Keep Foreman assets self‑contained under `.foreman/` so consumers can vendor them into their projects.
- Document how to wire OpenCode tools to concrete implementations (this repo uses OpenCode custom tools under `.opencode/tool/`).

---

## Tool contracts

These tools are generic and language‑agnostic. Implementations may vary (Python scripts are provided), but the tool interface (input/output) should match the schemas below.

### `validate_builder_result`

- Description: Validate a `builder_result` object or file against the Foreman builder contract.

- Input (JSON on stdin) — one of:
  - `{ "data": { <builder_result object> } }`
  - `{ "path": "builder_result.json" }` (relative path; defaults to `builder_result.json`)

- Output (JSON on stdout):

```json
{
  "ok": true | false,
  "errors": [
    { "path": "summary", "code": "required", "message": "summary is required" },
    ...
  ]
}
```

- Semantic contract (validators should enforce):
  - Root is a JSON object.
  - `run`: object
    - `status`: `"ok" | "failed"`
    - `failed_step`: string or null
    - `error`: string or null
  - `work`:
    - If `run.status === "ok"`: object with:
      - `summary`: non-empty string (recommend max length 300 characters)
      - `complexity`: one of `"low"`, `"medium"`, `"high"`
    - If `run.status === "failed"`: must be `null`


- Exit codes:
  - `0`: tool ran successfully (whether `ok` is true or false).
  - Non‑zero: unexpected infrastructure error (e.g. unreadable file, internal exception).


### `validate_inspector_result`

- Description: Validate an `inspector_result` object or file against the Foreman inspector contract.

- Input (JSON on stdin) — one of:
  - `{ "data": { <inspector_result object> } }`
  - `{ "path": "inspector_result.json" }` (relative path; defaults to `inspector_result.json`)

- Output (JSON on stdout): same envelope as above.

- Semantic contract (validators should enforce):
  - Root is a JSON object.
  - `run`: object
    - `status`: `"ok" | "failed"`
    - `failed_step`: string or null
    - `error`: string or null
  - `work`:
    - If `run.status === "ok"`: object with:
      - `status`: either `"approved"` or `"changes_requested"`
      - `issues`: array; when `status === "changes_requested"` it must be non-empty
        - Each issue must be an object:
          - `severity`: `"blocker" | "major" | "minor"`
          - `description`: non-empty string
          - `paths`: non-empty array of non-empty strings
      - `next_tasks`: array of strings (may be empty for `approved`)
    - If `run.status === "failed"`: must be `null`


- Exit codes: same semantics as `validate_builder_result`.

---

## Provided implementations (this repo)

In this repository, Foreman provides reference OpenCode tool implementations located under:

- `.opencode/tool/validate_builder_result.ts`
- `.opencode/tool/validate_inspector_result.ts`

These tools follow the JSON input/output contract described above and are intended as canonical, dependencylight implementations that agents can call directly via the OpenCode tool system.

---

## Example OpenCode custom tool wiring

The concrete OpenCode tool definition format will vary by runtime, but the essential idea is: define a tool that invokes a validator implementation matching the contracts above (for example, a custom tool under `.opencode/tool/`).

Example pseudoYAML (illustrative only):

```yaml
tools:
  - name: validate_builder_result
    description: Validate a builder_result object.
    # For example, wire this to `.opencode/tool/validate_builder_result.ts`
    implementation: opencode-custom-tool

  - name: validate_inspector_result
    description: Validate an inspector_result object.
    # Implementation can be a similar custom tool
    implementation: opencode-custom-tool
```


Notes for consumers:

- If your tool runtime supports JSON schemas for inputs/outputs, copy the schemas in the `Tool contracts` section into your tool definitions to provide early validation in the platform.
- Keep the `command` path relative to the consumer repo root so vendoring `.foreman/` into different repositories remains straightforward.

---

## Prompt snippets (to add to agent prompts)

Add these (or equivalent) instructions to the Builder and Inspector prompts so agent implementations MUST use the validators.

### Builder prompt snippet

- Before writing `builder_result.json` and finishing, the Builder MUST:
  1. Call the `validate_builder_result` tool with the object it intends to write (or with the `path` to the file it will write).
  2. If the tool returns `ok: false`, the Builder MUST correct its `builder_result` content and re‑run validation until `ok: true`.
  3. Once validation passes, write `builder_result.json` at the worktree root and finish with the mandatory handoff format.

Example (pseudocode):

```
# builder_result = { summary: ..., complexity: ... }
result = call_tool("validate_builder_result", { data: builder_result })
if not result.ok:
  fix builder_result according to result.errors
  repeat validation
else:
  write builder_result.json
  finish
```

### Inspector prompt snippet

- Before writing `inspector_result.json`, the Inspector MUST:
  1. Call `validate_inspector_result` with the object it intends to write.
  2. If `ok: false`, repair the object and re‑validate until `ok: true`.
  3. When passing, write `inspector_result.json` and return the final handoff.

---

## Generic skills (design)

Foreman defines (conceptual) skills that orchestrate the tools above. These live on the agent side and are optional, but useful to standardize higher‑level checks.

- `skill:check-builder-handoff`
  - Input: `data` or `path` for `builder_result`.
  - Behavior: calls `validate_builder_result`, optionally calls `git_status`.
  - Output: `{ status: "ok" | "issues", errors: [...] }`.

- `skill:check-inspector-handoff`
  - Same pattern using `validate_inspector_result`.

When implemented, skills should be packaged and documented so agent runtimes can discover them (for example, under an `.opencode/skill/` directory in the Foreman repo when extracted).

---

## Orchestration semantics (hard failures)

Foreman is designed to treat certain infrastructure failures as **hard stops** so downstream steps do not run on incomplete or inconsistent state.

- When a Builder or Inspector reports `run.status = "failed"`:
  - Foreman MUST stop the automation run.
  - Foreman MUST NOT attempt to create diffs, run Inspector, or open PRs.
  - A human can inspect the failed session output/logs, fix the underlying issue, and then re-run only the appropriate part of the workflow.

Common hard-failure examples:
- `run.failed_step = "pnpm install"`: dependencies could not be installed (often network restrictions). The agent should include exact error output in `run.error`.
- `run.failed_step = "git commit"`: a local commit could not be created, which breaks downstream diff/PR logic.

## Documentation & consumer wiring

When a consumer vendors `.foreman/`, they should:

- Point their OpenCode tool definitions to their chosen implementations that satisfy the contracts above (for example, custom tools under `.opencode/tool/`).
- Optionally implement the validators in their preferred language/runtime (Python, Bun, Node, etc.) as long as they obey the same JSON input/output envelopes.
- Add the prompt snippets (Builder/Inspector) into their agent prompts so the validators are used as part of the contract.

Add the following short example to your consumer README to make wiring trivial:

```text
OpenCode tool example (consumer):
- validate_builder_result -> an OpenCode tool that implements the Foreman builder_result contract
- validate_inspector_result -> an OpenCode tool that implements the Foreman inspector_result contract

Agent prompt guidance:
- Builder and Inspector prompts MUST call these tools before writing their result files.
```

---

## Next steps

- Define OpenCode tool definitions and add example YAML in this document (if you want a reference for a specific OpenCode runtime).
- Implement `git_status`, `git_diff_main_head`, `read_json_file`, and `write_json_file` generic tools if desired.
- Optionally implement the `skill:check-*` wrappers and publish them under `.opencode/skill/` when Foreman is moved to its own repo.

---

## Name suggestions (for discussion)

You mentioned preferring a gender‑neutral name. A few quick candidates (short, memorizable, and suggestive of orchestration):

- `ForeAI` — play on Foreman + AI (short and memorable).
- `Conductor` / `ConductorAI` — orchestrator metaphor.
- `ForgeAI` — hints at creating/forging changes.
- `Orchest` / `OrchestAI` — short for orchestrator.
- `ForeGuide` — less AI‑centric, indicates guidance.
- `Crewless` — playful, maybe less clear.
- `Tasksmith` — emphasizes crafting tasks.

Pick a direction and I’ll make a short branding + README update proposal when you’re ready.
