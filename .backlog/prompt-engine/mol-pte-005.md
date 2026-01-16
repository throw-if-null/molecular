# Create generic Carbon base template

**ID:** mol-pte-005  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Create a generic base template for Carbon (builder agent) that is stack-agnostic and can be customized per repository.

## Context

The Carbon template should define the agent's role, behavior constraints, and handoff procedures without being tied to specific technologies (pnpm, Go, Python, etc.). Repository-specific commands should be provided as placeholder sections that users customize.

## Implementation Details

### Files to Create

- `.molecular/carbon/prompt.md` - Main Carbon prompt template
- `.molecular/carbon/skills/checklist.md` - Generic implementation checklist
- `.molecular/carbon/skills/handoff-format.md` - Handoff procedure and JSON format

### Step-by-Step Instructions

1. **Create `.molecular/carbon/prompt.md`**

   ```markdown
   # Carbon Agent
   You are **Carbon**, the implementation agent for this repository.

   ## Your Role
   - Implement changes autonomously within the constraints defined in `AGENTS.md` (if present)
   - You MUST NOT expect human interaction during execution
   - If blocked, execute the Final Handoff Procedure and exit gracefully
   - You NEVER push to any remote or create PRs directly
   - You ALWAYS end your work by executing the Final Handoff Procedure

   ## Task Assignment
   {{task_prompt}}

   ## Environment
   - Task ID: `{{task_id}}`
   - Worktree: `{{worktree_path}}`
   - Artifacts: `{{artifacts_root}}`

   ## Implementation Checklist
   {{skill:checklist}}

   ## Final Handoff Procedure
   {{skill:handoff-format}}

   ---
   **CRITICAL:** No matter what happens (success, failure, blocked), you MUST execute the Final Handoff Procedure so that `builder_result.json` exists for Molecular to process.
   ```

2. **Create `.molecular/carbon/skills/checklist.md`**

   ```markdown
   Follow this implementation checklist in order:

   - [ ] (MANDATORY) Initialize a TODO list with `todowrite` (mirror this checklist)
   - [ ] Read `AGENTS.md` (if present) to understand repository constraints
   - [ ] Restate the assigned task briefly
   - [ ] Identify files and modules involved in the change
   - [ ] Implement required changes with minimal, focused diffs
   - [ ] Update or add tests for new/changed behavior

   <!-- REPOSITORY-SPECIFIC BUILD/TEST COMMANDS
   Customize this section for your tech stack.
   
   Examples:
     Go:     go mod tidy && go test ./... && golangci-lint run
     Python: pip install -r requirements.txt && pytest && mypy .
     Node:   npm install && npm run lint && npm test
     
   Delete this comment and add your commands below:
   -->

   - [ ] Run repository-specific build/test commands (see above)
   - [ ] Stage all relevant files with `git add`
   - [ ] Create a local commit with a clear, concise message
     - If `git commit` fails, treat it as a hard failure (set `run.status = "failed"`, `failed_step = "git commit"`, and proceed to handoff)
   - [ ] (CRITICAL) (MANDATORY) Execute the Final Handoff Procedure
   ```

3. **Create `.molecular/carbon/skills/handoff-format.md`**

   ```markdown
   ## Final Handoff Procedure (MANDATORY)

   When your task is complete (or you are blocked), execute this procedure:

   ### 1. Public Handoff (Chat Output)
   Print a message with these sections:

   1. **Summary**
      - 1–3 short bullets describing what you implemented/changed

   2. **Files Touched**
      - Bullet list of paths modified or created

   3. **Commands Run + Results**
      - List relevant commands and pass/fail status (or why skipped)

   4. **Notes / Risks**
      - Known limitations, edge cases, or follow-up work

   5. **Checklist (TODO Snapshot)**
      - Print the current `todowrite` TODO list (all items + final statuses)
      - Do NOT save to disk; must be visible in final message

   ### 2. Molecular Handoff (JSON File)
   Write a file named `builder_result.json` in the worktree root:

   \```json
   {
     "run": {
       "status": "ok" | "failed",
       "failed_step": "..." | null,
       "error": "..." | null
     },
     "work": {
       "summary": "short natural-language summary",
       "complexity": "low" | "medium" | "high"
     } | null
   }
   \```

   **Rules:**
   - If `run.status` is `ok`, `work` MUST be an object
   - If `run.status` is `failed`, `work` MUST be `null`
   - Run `validate_builder_result` (if available) and fix issues until it passes

   **Complexity Guidelines:**
   - `low`: trivial/small change, docs-only, localized fix
   - `medium`: non-trivial logic with limited scope
   - `high`: public API changes, cross-cutting behavior, significant infrastructure changes
   ```

4. **Create directory structure**
   ```bash
   mkdir -p .molecular/carbon/skills
   ```

5. **Write all three files to disk**

6. **Verify templates render correctly**
   ```bash
   # Will be tested properly once molecular doctor is updated
   # For now, manually check that files exist and have correct content
   ls -la .molecular/carbon/
   cat .molecular/carbon/prompt.md
   ```

## Testing

- [ ] Manual verification: files created in correct locations
- [ ] Manual verification: prompt.md contains skill references
- [ ] Manual verification: skills exist and have sensible content
- [ ] Template loads successfully (will be tested in mol-pte-007)

## Acceptance Criteria

- [ ] `.molecular/carbon/prompt.md` created with generic content
- [ ] `.molecular/carbon/skills/checklist.md` created
- [ ] `.molecular/carbon/skills/handoff-format.md` created
- [ ] Template uses variable substitution: `{{task_id}}`, `{{task_prompt}}`, etc.
- [ ] Template references skills: `{{skill:checklist}}`, `{{skill:handoff-format}}`
- [ ] No stack-specific commands in base template (pnpm, go, etc.)
- [ ] Clear placeholder for users to add their own build/test commands
- [ ] Files are properly formatted Markdown

## Dependencies

None - this task is independent (creates new files).

## Notes

- This template is a starting point - users should customize the checklist section
- The handoff procedure (JSON format) is universal and should not change
- Consider creating example templates for common stacks (Go, Python, Node) in docs
