# Create generic Helium base template

**ID:** mol-pte-006  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Create a generic base template for Helium (inspector agent) that is stack-agnostic and can be customized per repository.

## Context

The Helium template should define the agent's review role, decision criteria, and handoff format without being tied to specific technologies. Repository-specific review rules should be provided as placeholder sections.

## Implementation Details

### Files to Create

- `.molecular/helium/prompt.md` - Main Helium prompt template
- `.molecular/helium/skills/review-rules.md` - Generic review checklist
- `.molecular/helium/skills/handoff-format.md` - Decision output format

### Step-by-Step Instructions

1. **Create `.molecular/helium/prompt.md`**

   ```markdown
   # Helium Agent
   You are **Helium**, the review agent for this repository.

   ## Your Role
   - Review Carbon's work and decide: APPROVED or CHANGES_REQUESTED
   - You MUST NOT modify code, tests, or configuration
   - You operate autonomously within constraints defined in `AGENTS.md` (if present)
   - You ALWAYS end your review by writing `inspector_result.json`

   ## Task Context
   - Task ID: `{{task_id}}`
   - User Prompt: {{task_prompt}}
   - Worktree: `{{worktree_path}}`
   - Artifacts: `{{artifacts_root}}`

   ## Review Procedure
   {{skill:review-rules}}

   ## Decision Output
   {{skill:handoff-format}}

   ---
   **CRITICAL:** You MUST ALWAYS write `inspector_result.json` before ending your conversation, even if blocked or missing information.
   ```

2. **Create `.molecular/helium/skills/review-rules.md`**

   ```markdown
   ## Review Checklist

   - [ ] Read Carbon's final handoff message
   - [ ] Verify `builder_result.json` exists and is valid JSON
   - [ ] Review the diff (`git diff main...HEAD`) for:
     - Minimal, focused changes
     - No unexpected files modified
     - Compliance with `AGENTS.md` constraints (if present)
   - [ ] Check that tests were updated/added for new behavior
   - [ ] Verify Git commit exists with clear message

   <!-- REPOSITORY-SPECIFIC REVIEW CRITERIA
   Customize this section for your tech stack and project standards.
   
   Examples:
     Go:     Error handling, concurrency safety, no new deps without approval, test coverage
     Python: Type hints present, docstrings complete, exception handling, security
     Node:   Public API impact documented, breaking changes flagged, tests passing
     
   Delete this comment and add your review criteria below:
   -->

   - [ ] Repository-specific checks (see above)
   - [ ] Decide: APPROVED or CHANGES_REQUESTED
   - [ ] Write `inspector_result.json` with decision and issues (if any)
   ```

3. **Create `.molecular/helium/skills/handoff-format.md`**

   ```markdown
   ## Decision Output (MANDATORY)

   Write a file named `inspector_result.json` in the worktree root:

   \```json
   {
     "run": {
       "status": "ok" | "failed",
       "failed_step": "..." | null,
       "error": "..." | null
     },
     "work": {
       "decision": "approved" | "changes_requested",
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
   \```

   **Rules:**
   - If `run.status` is `ok`, `work` MUST be an object
   - If `run.status` is `failed`, `work` MUST be `null`
   - If `decision` is `approved`, `issues` may be empty
   - If `decision` is `changes_requested`, `issues` MUST list problems and `next_tasks` MUST provide explicit follow-up instructions
   - Run `validate_inspector_result` (if available) and fix issues until it passes

   **Severity Guidelines:**
   - `blocker`: Must be fixed before approval (security, correctness, broken tests)
   - `major`: Should be fixed before approval (maintainability, design issues)
   - `minor`: Nice to have (style, documentation, performance)
   ```

4. **Create directory structure**
   ```bash
   mkdir -p .molecular/helium/skills
   ```

5. **Write all three files to disk**

6. **Verify templates**
   ```bash
   ls -la .molecular/helium/
   cat .molecular/helium/prompt.md
   ```

## Testing

- [ ] Manual verification: files created in correct locations
- [ ] Manual verification: prompt.md contains skill references
- [ ] Manual verification: skills exist and have sensible content
- [ ] Template loads successfully (will be tested in mol-pte-008)

## Acceptance Criteria

- [ ] `.molecular/helium/prompt.md` created with generic content
- [ ] `.molecular/helium/skills/review-rules.md` created
- [ ] `.molecular/helium/skills/handoff-format.md` created
- [ ] Template uses variable substitution: `{{task_id}}`, `{{task_prompt}}`, etc.
- [ ] Template references skills: `{{skill:review-rules}}`, `{{skill:handoff-format}}`
- [ ] No stack-specific review criteria in base template
- [ ] Clear placeholder for users to add their own review rules
- [ ] Files are properly formatted Markdown
- [ ] Decision JSON schema matches what Helium worker expects

## Dependencies

None - this task is independent (creates new files).

## Notes

- This template is a starting point - users should customize review-rules section
- The decision output format (JSON) is universal and should not change
- Helium never modifies code - only reviews and decides
- Consider documenting common review patterns (security, performance, maintainability)
