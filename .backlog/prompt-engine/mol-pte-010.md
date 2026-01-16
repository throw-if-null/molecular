# Migrate .foreman templates to .molecular

**ID:** mol-pte-010  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Migrate existing `.foreman/` templates to the new `.molecular/` template system as concrete examples, customized for this repository's Go/pnpm workflow.

## Context

The `.foreman/` directory contains Python-based templates with FEC-specific commands (pnpm, Svelte checks). This task:
1. Extracts the useful patterns from `.foreman/`
2. Adapts them to Molecular's Go-based template system
3. Customizes for this repo's actual workflow (Go tests, lint, etc.)
4. Serves as a reference implementation for users

## Implementation Details

### Files to Create/Modify

- `.molecular/carbon/prompt.md` - Already exists from mol-pte-005, customize for this repo
- `.molecular/carbon/skills/checklist.md` - Customize with Go-specific commands
- `.molecular/helium/prompt.md` - Already exists from mol-pte-006, customize for this repo
- `.molecular/helium/skills/review-rules.md` - Customize with Go-specific review criteria

### Step-by-Step Instructions

1. **Customize Carbon checklist for this repository**

   Update `.molecular/carbon/skills/checklist.md`:
   
   ```markdown
   Follow this implementation checklist in order:

   - [ ] (MANDATORY) Initialize a TODO list with `todowrite` (mirror this checklist)
   - [ ] Read `AGENTS.md` (if present) to understand repository constraints
   - [ ] Restate the assigned task briefly
   - [ ] Identify files and modules involved in the change
   - [ ] Implement required changes with minimal, focused diffs
   - [ ] Update or add tests for new/changed behavior

   <!-- Build/Test Commands for Molecular (Go project) -->
   - [ ] (MANDATORY) Run `go mod tidy` to ensure dependencies are correct
   - [ ] (MANDATORY) Run `go test ./...` and verify all tests pass
   - [ ] (OPTIONAL) Run `golangci-lint run` if available (record result)
   - [ ] (MANDATORY) Run `go build ./cmd/silicon ./cmd/molecular` to verify builds

   - [ ] Stage all relevant files with `git add`
   - [ ] Create a local commit with a clear, concise message
     - If `git commit` fails, treat it as a hard failure (set `run.status = "failed"`, `failed_step = "git commit"`, and proceed to handoff)
   - [ ] (CRITICAL) (MANDATORY) Execute the Final Handoff Procedure
   ```

2. **Customize Helium review rules for this repository**

   Update `.molecular/helium/skills/review-rules.md`:
   
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

   <!-- Repository-Specific Review Criteria for Molecular (Go project) -->
   - [ ] Go code quality:
     - Proper error handling (no ignored errors)
     - Clear, descriptive variable/function names
     - Appropriate use of contexts for cancellation
     - No goroutine leaks
   - [ ] Test coverage:
     - New functions have corresponding tests
     - Tests use table-driven design where appropriate
     - Edge cases covered
   - [ ] Dependencies:
     - No new external dependencies added without approval
     - `go.mod` and `go.sum` updated via `go mod tidy`
   - [ ] Architecture:
     - Changes align with existing package boundaries
     - Clean separation between API, store, and worker layers
     - No circular dependencies introduced
   - [ ] Documentation:
     - Public functions have clear comments
     - README updated if user-facing behavior changed
     - Breaking changes clearly noted

   - [ ] Decide: APPROVED or CHANGES_REQUESTED
   - [ ] Write `inspector_result.json` with decision and issues (if any)
   ```

3. **Add custom skills from .foreman/ if useful**

   Review `.foreman/builder/builder.prompt.md` and `.opencode/skill/builder-signoff/SKILL.md`:
   - Extract any useful patterns not yet in generic templates
   - Create additional skill files if needed (e.g., `git-workflow.md`, `testing-guidelines.md`)

4. **Archive .foreman/ directory**

   ```bash
   # Don't delete .foreman/ yet - keep as reference
   # Add note to .foreman/README.md:
   echo "NOTE: This directory is legacy. See .molecular/ for current templates." >> .foreman/README.md
   ```

5. **Update main README.md to document templates**

   Add a "Templates" section to README.md:
   
   ```markdown
   ## Templates

   Molecular uses prompt templates to provide repository-specific instructions to Carbon (builder) and Helium (inspector) agents.

   Templates are located in `.molecular/<agent>/`:
   - `prompt.md` - Main agent prompt (mandatory)
   - `skills/*.md` - Reusable instruction snippets (optional)

   Templates support:
   - Variable substitution: `{{task_id}}`, `{{task_prompt}}`, etc.
   - Skill composition: `{{skill:checklist}}` inlines skill content

   Use `molecular doctor` to validate your templates.

   See `.molecular/carbon/` and `.molecular/helium/` for examples.
   ```

6. **Verify migrated templates work end-to-end**

   ```bash
   # Start Silicon
   go run ./cmd/silicon

   # Submit a test task
   go run ./cmd/molecular submit --task-id test-migrate --prompt "Add a comment to version.go"

   # Check that templates were rendered in attempt artifacts
   cat .molecular/runs/test-migrate/attempts/*/prompt.txt

   # Verify Carbon and Helium executed successfully
   go run ./cmd/molecular status test-migrate
   ```

## Testing

- [ ] Manually verify customized templates render correctly
- [ ] End-to-end test: Submit task, verify templates used
- [ ] Check `prompt.txt` in artifacts contains rendered template
- [ ] Verify Carbon follows Go-specific checklist (go test, go mod tidy)
- [ ] Verify Helium uses Go-specific review criteria
- [ ] `molecular doctor` reports templates as valid

## Acceptance Criteria

- [ ] Carbon template customized for Go workflow (go test, go mod tidy, go build)
- [ ] Helium template customized for Go review criteria
- [ ] Templates render successfully with real task variables
- [ ] End-to-end task execution uses customized templates
- [ ] README.md updated with Templates section
- [ ] `.foreman/` directory noted as legacy (not deleted, kept for reference)
- [ ] All useful patterns from `.foreman/` extracted and adapted
- [ ] `molecular doctor` validates migrated templates successfully

## Dependencies

- Depends on: [mol-pte-007](./mol-pte-007.md) - Carbon worker integration
- Depends on: [mol-pte-008](./mol-pte-008.md) - Helium worker integration
- Depends on: [mol-pte-009](./mol-pte-009.md) - Doctor validation

## Notes

- This is the final integration task that proves the whole system works
- Keep `.foreman/` as reference (don't delete) in case we need to refer back
- The customized templates serve as examples for other users/repos
- Consider creating a "template migration guide" for users coming from Foreman
- Verify that all `.foreman/` patterns are captured (checklist, handoff format, skills)
