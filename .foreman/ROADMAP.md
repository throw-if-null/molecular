# ROADMAP

This roadmap tracks improvements to the agent workflow, tools, and
Foreman orchestration for this repository (and, by extension, projects
that reuse these prompts/templates).

## Agent Tools

### Result contract validators

- [x] **Implement `builder_result.json` validator**  
  Implement a lightweight validator for the Builder JSON contract:
  - Ensure the file exists at the worktree root.
  - Validate that it contains exactly one JSON object with:
    - `summary`: non-empty string, reasonably short.
    - `complexity`: one of `"low"`, `"medium"`, `"high"`.
  - Return structured errors (missing field, wrong type, invalid value)
    so Builder can fix and rewrite the file.

- [x] **Implement `inspector_result.json` validator**  
  Implement a lightweight validator for the Inspector JSON contract:
  - Ensure the file exists at the worktree root.
  - Validate that it contains exactly one JSON object with:
    - `status`: `"approved"` or `"changes_requested"`.
    - `issues`: array; if `status === "changes_requested"`, must be
      non-empty.
      - Each issue has:
        - `severity`: `"blocker"`, `"major"`, or `"minor"`.
        - `description`: non-empty string.
        - `paths`: array of non-empty strings.
    - `next_tasks`: array of strings (may be empty when `status` is
      `"approved"`).
  - Return structured errors so Inspector can repair the file and
    re-validate.

- [x] **Expose validators as generic tools**  
  Expose these validators as tools that Builder/Inspector can call
  from their prompts (for example, via MCP, a small CLI, or a local
  opencode tool), and update prompts to recommend running validation
  after writing the files.

### Generic OpenCode tools (repo-agnostic)

- [x] **Implement `validate_builder_result` tool**  
  Generic tool that validates a `builder_result` object against the
  public contract and returns structured errors.

- [x] **Implement `validate_inspector_result` tool**  
  Generic tool that validates an `inspector_result` object against the
  public contract and returns structured errors.

- [x] **Implement `read_json_file` tool**  
  Generic tool that reads and parses a JSON file given a relative path
  and returns the resulting object.

- [x] **Implement `write_json_file` tool**  
  Generic tool that writes a JSON object to a relative path with
  stable formatting.

### Generic OpenCode skills (repo-agnostic)

- [x] **Implement `skill:check-builder-handoff`**  
  Generic skill that orchestrates tools (for example, `validate_builder_result`)
  to validate a Builder handoff and return a single
  verdict plus issues.

- [x] **Implement `skill:check-inspector-handoff`**  
  Generic skill that orchestrates tools (for example, `validate_inspector_result`)
  to validate an Inspector handoff and return a single
  verdict plus issues.

## Foreman Improvements

- [ ] **Implement a feedback loop between Inspector and Builder**  
  Enhance the Foreman workflow so that `changes_requested` can
  automatically trigger follow-up Builder iterations instead of
  terminating the run:
  - When `inspector_result.json.status === "changes_requested"`:
    - Record the issues and `next_tasks` in a structured way (for
      example, in a task log or monitoring system).
    - Start a new Builder iteration in the same worktree, feeding the
      `next_tasks` list and relevant context back into the Builder
      prompt as the next assignment.
    - After Builder completes and updates the worktree, run Inspector
      again with an updated diff and context.
  - Continue this loop until Inspector returns `"approved"` or a
    configurable maximum number of iterations is reached.
  - Ensure each loop iteration still relies solely on
    `builder_result.json` and `inspector_result.json` for structured
    state, keeping chat output as human-facing only.
