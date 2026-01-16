# Integrate templates into Helium worker

**ID:** mol-pte-008  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Integrate template loading and rendering into the Helium worker execution path so that Helium receives a fully rendered prompt before reviewing Carbon's work.

## Context

Similar to mol-pte-007 but for the Helium (inspector) worker. Helium needs review guidelines, decision criteria, and output format instructions from the template.

## Implementation Details

### Files to Modify

- `internal/silicon/worker.go` - Add template rendering to Helium attempt setup (or wherever Helium attempts are executed)

### Step-by-Step Instructions

1. **Import templates package if not already imported**

   ```go
   import (
       "github.com/throw-if-null/molecular/internal/templates"
   )
   ```

2. **Locate Helium attempt execution function**
   
   Find the function that handles Helium attempts (likely `runHeliumAttempt` or similar).

3. **Add template loading and rendering before command execution**

   ```go
   // Load Helium template
   tmpl, err := templates.Load(templates.Config{
       RepoRoot: repoRoot,
       Role:     "helium",
   })
   if err != nil {
       // Template loading failure is FATAL - Helium cannot proceed without review guidelines
       errMsg := fmt.Sprintf("Failed to load Helium template: %v", err)
       writeLog(logPath, errMsg+"\n")
       _ = s.UpdateAttemptStatus(attemptID, "failed", errMsg)
       _ = s.UpdateTaskPhaseAndStatus(t.TaskID, "review", "failed")
       return
   }

   // Render template with task variables
   vars := templates.Variables{
       "task_id":        t.TaskID,
       "task_prompt":    t.Prompt,
       "role":           "helium",
       "repo_root":      repoRoot,
       "worktree_path":  t.WorktreePath,
       "artifacts_root": t.ArtifactsRoot,
   }
   renderedPrompt, err := tmpl.Render(vars)
   if err != nil {
       errMsg := fmt.Sprintf("Failed to render Helium template: %v", err)
       writeLog(logPath, errMsg+"\n")
       _ = s.UpdateAttemptStatus(attemptID, "failed", errMsg)
       _ = s.UpdateTaskPhaseAndStatus(t.TaskID, "review", "failed")
       return
   }

   // Write rendered prompt to artifacts for debugging
   promptPath := filepath.Join(artifactsDir, "prompt.txt")
   if err := os.WriteFile(promptPath, []byte(renderedPrompt), 0644); err != nil {
       writeLog(logPath, fmt.Sprintf("Warning: failed to write prompt.txt: %v\n", err))
   }

   // Log template info
   writeLog(logPath, fmt.Sprintf("Loaded template from %s\n", filepath.Join(repoRoot, ".molecular/helium/prompt.md")))
   writeLog(logPath, fmt.Sprintf("Skills loaded: %v\n", tmpl.Skills()))
   ```

4. **Pass rendered prompt to Helium command**

   Similar to Carbon, decide how to pass the prompt:
   - File: `--prompt-file=/path/to/prompt.txt`
   - Stdin: pipe prompt to command
   - Environment variable

   Example (via file):
   ```go
   cmdArgs = append(cmdArgs, "--prompt-file", promptPath)
   ```

5. **Update Helium command invocation in config defaults**

   Ensure Helium command config supports receiving a prompt file.

6. **Test with existing Helium worker tests**
   
   Update tests in `internal/silicon/helium_test.go`:
   - Create minimal test templates
   - Verify template loading doesn't break tests
   - Add test for missing template

## Testing

- [ ] Unit test: Helium worker with valid template renders correctly
- [ ] Unit test: Helium worker with missing template fails with clear error
- [ ] Integration test: End-to-end Helium execution with template
- [ ] Manual test: Run Silicon with real `.molecular/helium/` template
- [ ] Verify `prompt.txt` written to Helium artifacts
- [ ] Verify template load errors are logged clearly

## Acceptance Criteria

- [ ] Template loading added to Helium worker execution path
- [ ] Templates.Load called before command execution
- [ ] Rendered prompt written to `<artifacts_dir>/prompt.txt`
- [ ] Template variables correctly populated (task_id, task_prompt, etc.)
- [ ] Missing template causes graceful failure with clear error
- [ ] Template render errors cause graceful failure with clear error
- [ ] Existing Helium tests still pass (or updated appropriately)
- [ ] Helium command receives rendered prompt (via file, stdin, or env var)

## Dependencies

- Depends on: [mol-pte-003](./mol-pte-003.md) - Render function must exist
- Depends on: [mol-pte-006](./mol-pte-006.md) - Helium template must exist

## Notes

- Similar to Carbon integration (mol-pte-007) but for Helium
- Helium needs access to Carbon's result (`builder_result.json`) - ensure this is still accessible
- Template loading should happen per-attempt
- Consider sharing template loading/rendering logic between Carbon and Helium workers (DRY)
- Write rendered prompt to artifacts for debugging/auditability
