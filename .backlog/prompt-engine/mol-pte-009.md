# Add template validation to molecular doctor

**ID:** mol-pte-009  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Extend the `molecular doctor` command to validate that Carbon and Helium templates exist, are readable, and can be rendered successfully.

## Context

The `molecular doctor` command already checks for git, gh, config.toml, and hook scripts. This task adds template validation so users can verify their template setup before running tasks.

## Implementation Details

### Files to Modify

- `cmd/molecular/main.go` - Add template checks to `doctorWithIO` function

### Step-by-Step Instructions

1. **Import templates package**

   ```go
   import (
       "github.com/throw-if-null/molecular/internal/templates"
   )
   ```

2. **Add template fields to doctor report struct**

   ```go
   type report struct {
       Git          bool     `json:"git"`
       GH           bool     `json:"gh"`
       Config       bool     `json:"config"`
       Hooks        []string `json:"hooks"`
       CarbonTemplate  bool     `json:"carbon_template"`
       HeliumTemplate  bool     `json:"helium_template"`
       Problems     []string `json:"problems"`
   }
   ```

3. **Add Carbon template check in doctorWithIO**

   ```go
   // Check Carbon template
   repoRoot, _ := os.Getwd()
   carbonTmpl, err := templates.Load(templates.Config{RepoRoot: repoRoot, Role: "carbon"})
   if err != nil {
       res.CarbonTemplate = false
       res.Problems = append(res.Problems, fmt.Sprintf("Carbon template: %v", err))
   } else {
       // Try rendering with dummy variables to validate syntax
       dummyVars := templates.Variables{
           "task_id":        "test-001",
           "task_prompt":    "Test task",
           "role":           "carbon",
           "repo_root":      repoRoot,
           "worktree_path":  "/tmp/test",
           "artifacts_root": "/tmp/artifacts",
       }
       _, err := carbonTmpl.Render(dummyVars)
       if err != nil {
           res.CarbonTemplate = false
           res.Problems = append(res.Problems, fmt.Sprintf("Carbon template render: %v", err))
       } else {
           res.CarbonTemplate = true
       }
   }
   ```

4. **Add Helium template check**

   ```go
   // Check Helium template
   heliumTmpl, err := templates.Load(templates.Config{RepoRoot: repoRoot, Role: "helium"})
   if err != nil {
       res.HeliumTemplate = false
       res.Problems = append(res.Problems, fmt.Sprintf("Helium template: %v", err))
   } else {
       // Try rendering with dummy variables
       dummyVars := templates.Variables{
           "task_id":        "test-001",
           "task_prompt":    "Test task",
           "role":           "helium",
           "repo_root":      repoRoot,
           "worktree_path":  "/tmp/test",
           "artifacts_root": "/tmp/artifacts",
       }
       _, err := heliumTmpl.Render(dummyVars)
       if err != nil {
           res.HeliumTemplate = false
           res.Problems = append(res.Problems, fmt.Sprintf("Helium template render: %v", err))
       } else {
           res.HeliumTemplate = true
       }
   }
   ```

5. **Update human-friendly output section**

   ```go
   // In the human-friendly output section
   fmt.Fprintf(out, "  carbon template: %v\n", res.CarbonTemplate)
   fmt.Fprintf(out, "  helium template: %v\n", res.HeliumTemplate)
   ```

6. **Update help text to mention template validation**

   Update the `usage` function to mention that `doctor` checks templates:
   
   ```go
   _, _ = fmt.Fprintln(w, "doctor checks:")
   _, _ = fmt.Fprintln(w, "  - git in PATH (required)")
   _, _ = fmt.Fprintln(w, "  - gh in PATH (optional)")
   _, _ = fmt.Fprintln(w, "  - .molecular/config.toml exists")
   _, _ = fmt.Fprintln(w, "  - .molecular/carbon/prompt.md exists and renders")
   _, _ = fmt.Fprintln(w, "  - .molecular/helium/prompt.md exists and renders")
   _, _ = fmt.Fprintln(w, "  - .molecular/lithium.sh and .molecular/chlorine.sh exist + executable")
   ```

7. **Test doctor command**

   ```bash
   # With templates present
   go run ./cmd/molecular doctor
   
   # With missing templates
   rm -rf .molecular/carbon
   go run ./cmd/molecular doctor  # Should report problems
   
   # With invalid template syntax
   echo "{{skill:missing}}" >> .molecular/carbon/prompt.md
   go run ./cmd/molecular doctor  # Should report undefined skill
   ```

## Testing

- [ ] Unit test for doctor with valid templates (all checks pass)
- [ ] Unit test for doctor with missing Carbon template (problem reported)
- [ ] Unit test for doctor with missing Helium template (problem reported)
- [ ] Unit test for doctor with template render error (undefined skill/var)
- [ ] Manual test: Run `molecular doctor` with valid setup
- [ ] Manual test: Run `molecular doctor` with missing templates
- [ ] Manual test: Run `molecular doctor --json` produces valid JSON

## Acceptance Criteria

- [ ] `molecular doctor` checks for Carbon template existence
- [ ] `molecular doctor` checks for Helium template existence
- [ ] `molecular doctor` validates templates can be rendered (syntax check)
- [ ] Missing templates are reported as problems
- [ ] Template render errors are reported with clear messages
- [ ] JSON output includes `carbon_template` and `helium_template` fields
- [ ] Human-friendly output shows template check status
- [ ] Help text updated to mention template checks
- [ ] Exit code 1 if templates are missing/invalid
- [ ] All tests passing

## Dependencies

- Depends on: [mol-pte-003](./mol-pte-003.md) - Render function must exist
- Depends on: [mol-pte-005](./mol-pte-005.md) - Carbon template should exist for testing
- Depends on: [mol-pte-006](./mol-pte-006.md) - Helium template should exist for testing

## Notes

- Use dummy variables for render validation (just checking syntax, not actual execution)
- Template checks should be non-fatal to doctor command itself (just report problems)
- Consider showing skill count in output: "carbon template: ok (3 skills loaded)"
- This provides early feedback before users try to run tasks
