# Integrate templates into Carbon worker

**ID:** mol-pte-007  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Integrate template loading and rendering into the Carbon worker execution path so that Carbon receives a fully rendered prompt before starting work.

## Context

The Carbon worker currently executes commands with minimal context. This task adds template rendering to provide Carbon with:
- Repository-specific instructions (via `.molecular/carbon/prompt.md`)
- Task context (task_id, task_prompt, worktree_path, etc.)
- Implementation checklist and handoff procedures

## Implementation Details

### Files to Modify

- `internal/silicon/worker.go` - Add template rendering to Carbon attempt setup
- `cmd/silicon/main.go` - Verify template loading works on Silicon startup (optional early validation)

### Step-by-Step Instructions

1. **Import templates package in worker.go**

   ```go
   import (
       "github.com/throw-if-null/molecular/internal/templates"
   )
   ```

2. **Locate Carbon attempt execution function**
   
   Find the function that handles Carbon attempts (likely `runCarbonAttempt` or similar).

3. **Add template loading and rendering before command execution**

   ```go
   // Load Carbon template
   tmpl, err := templates.Load(templates.Config{
       RepoRoot: repoRoot,
       Role:     "carbon",
   })
   if err != nil {
       // Template loading failure is FATAL - Carbon cannot proceed without instructions
       errMsg := fmt.Sprintf("Failed to load Carbon template: %v", err)
       writeLog(logPath, errMsg+"\n")
       _ = s.UpdateAttemptStatus(attemptID, "failed", errMsg)
       _ = s.UpdateTaskPhaseAndStatus(t.TaskID, "build", "failed")
       return
   }

   // Render template with task variables
   vars := templates.Variables{
       "task_id":        t.TaskID,
       "task_prompt":    t.Prompt,
       "role":           "carbon",
       "repo_root":      repoRoot,
       "worktree_path":  t.WorktreePath,
       "artifacts_root": t.ArtifactsRoot,
   }
   renderedPrompt, err := tmpl.Render(vars)
   if err != nil {
       errMsg := fmt.Sprintf("Failed to render Carbon template: %v", err)
       writeLog(logPath, errMsg+"\n")
       _ = s.UpdateAttemptStatus(attemptID, "failed", errMsg)
       _ = s.UpdateTaskPhaseAndStatus(t.TaskID, "build", "failed")
       return
   }

   // Write rendered prompt to artifacts for debugging
   promptPath := filepath.Join(artifactsDir, "prompt.txt")
   if err := os.WriteFile(promptPath, []byte(renderedPrompt), 0644); err != nil {
       writeLog(logPath, fmt.Sprintf("Warning: failed to write prompt.txt: %v\n", err))
   }

   // Log template info (help debugging)
   writeLog(logPath, fmt.Sprintf("Loaded template from %s\n", filepath.Join(repoRoot, ".molecular/carbon/prompt.md")))
   writeLog(logPath, fmt.Sprintf("Skills loaded: %v\n", tmpl.Skills()))
   ```

4. **Pass rendered prompt to Carbon command**

   Depending on how the Carbon command is invoked, you may need to:
   - Write prompt to a file and pass path as arg: `--prompt-file=/path/to/prompt.txt`
   - Pass via stdin (pipe rendered prompt to command)
   - Pass as environment variable: `CARBON_PROMPT=<rendered>`

   Example (via file):
   ```go
   // Already written prompt to promptPath above
   // Add to command args
   cmdArgs = append(cmdArgs, "--prompt-file", promptPath)
   ```

5. **Update Carbon command invocation in config defaults**

   If Carbon command needs to know about prompt file location, ensure config default includes the right flags.

6. **Test with existing Carbon worker tests**
   
   Update tests in `internal/silicon/carbon_test.go` to:
   - Create minimal test templates in temp directories
   - Verify template loading doesn't break existing tests
   - Add test case for missing template (should fail gracefully)

## Testing

- [ ] Unit test: Carbon worker with valid template renders correctly
- [ ] Unit test: Carbon worker with missing template fails with clear error
- [ ] Integration test: End-to-end Carbon execution with template
- [ ] Manual test: Run Silicon with real `.molecular/carbon/` template
- [ ] Verify `prompt.txt` is written to artifacts directory
- [ ] Verify template load errors are logged clearly

## Acceptance Criteria

- [ ] Template loading added to Carbon worker execution path
- [ ] Templates.Load called before command execution
- [ ] Rendered prompt written to `<artifacts_dir>/prompt.txt`
- [ ] Template variables correctly populated (task_id, task_prompt, etc.)
- [ ] Missing template causes graceful failure with clear error
- [ ] Template render errors cause graceful failure with clear error
- [ ] Existing Carbon tests still pass (or updated appropriately)
- [ ] Carbon command receives rendered prompt (via file, stdin, or env var)

## Dependencies

- Depends on: [mol-pte-003](./mol-pte-003.md) - Render function must exist
- Depends on: [mol-pte-005](./mol-pte-005.md) - Carbon template must exist

## Notes

- Decide how to pass prompt to Carbon command (file, stdin, env var)
- Ensure existing Carbon command interface supports receiving a prompt
- Template loading should happen per-attempt (not once at startup) in case templates change
- Consider caching loaded templates if performance becomes an issue
- Write rendered prompt to artifacts for debugging/auditability
