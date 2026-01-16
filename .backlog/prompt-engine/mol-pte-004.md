# Add template unit tests

**ID:** mol-pte-004  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Create comprehensive unit tests for the templates package covering Load, Render, variable substitution, skill composition, and error cases.

## Context

Solid test coverage ensures the template system works correctly across various scenarios including edge cases and error conditions. Tests should validate both happy paths and failure modes.

## Implementation Details

### Files to Create

- `internal/templates/templates_test.go` - Unit tests

### Step-by-Step Instructions

1. **Create test file with table-driven tests**

   ```go
   package templates

   import (
       "os"
       "path/filepath"
       "testing"
   )

   func TestLoad(t *testing.T) {
       tests := []struct {
           name      string
           setup     func(dir string)
           role      string
           wantErr   bool
           errContains string
       }{
           {
               name: "valid carbon template with skills",
               role: "carbon",
               setup: func(dir string) {
                   os.MkdirAll(filepath.Join(dir, ".molecular/carbon/skills"), 0755)
                   os.WriteFile(filepath.Join(dir, ".molecular/carbon/prompt.md"), []byte("# Carbon {{task_id}}"), 0644)
                   os.WriteFile(filepath.Join(dir, ".molecular/carbon/skills/test.md"), []byte("Test skill"), 0644)
               },
               wantErr: false,
           },
           {
               name: "missing prompt.md",
               role: "carbon",
               setup: func(dir string) {
                   os.MkdirAll(filepath.Join(dir, ".molecular/carbon"), 0755)
               },
               wantErr: true,
               errContains: "not found",
           },
           {
               name: "invalid role",
               role: "invalid",
               setup: func(dir string) {},
               wantErr: true,
               errContains: "invalid role",
           },
           {
               name: "no skills directory (should not error)",
               role: "helium",
               setup: func(dir string) {
                   os.MkdirAll(filepath.Join(dir, ".molecular/helium"), 0755)
                   os.WriteFile(filepath.Join(dir, ".molecular/helium/prompt.md"), []byte("# Helium"), 0644)
               },
               wantErr: false,
           },
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               dir := t.TempDir()
               tt.setup(dir)

               tmpl, err := Load(Config{RepoRoot: dir, Role: tt.role})
               if (err != nil) != tt.wantErr {
                   t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
                   return
               }
               if tt.wantErr && tt.errContains != "" {
                   if err == nil || !contains(err.Error(), tt.errContains) {
                       t.Errorf("Load() error = %v, want error containing %q", err, tt.errContains)
                   }
               }
               if !tt.wantErr && tmpl == nil {
                   t.Error("Load() returned nil template without error")
               }
           })
       }
   }

   func TestRender(t *testing.T) {
       tests := []struct {
           name    string
           prompt  string
           skills  map[string]string
           vars    Variables
           want    string
           wantErr bool
           errContains string
       }{
           {
               name:   "simple variable substitution",
               prompt: "Task: {{task_id}}",
               vars:   Variables{"task_id": "test-123"},
               want:   "Task: test-123",
           },
           {
               name:   "multiple variables",
               prompt: "{{task_id}} - {{task_prompt}}",
               vars:   Variables{"task_id": "t1", "task_prompt": "Build feature"},
               want:   "t1 - Build feature",
           },
           {
               name:   "skill substitution",
               prompt: "# Header\n{{skill:intro}}",
               skills: map[string]string{"intro": "Introduction text"},
               vars:   Variables{},
               want:   "# Header\nIntroduction text",
           },
           {
               name:   "recursive skills",
               prompt: "{{skill:outer}}",
               skills: map[string]string{
                   "outer": "Outer {{skill:inner}}",
                   "inner": "Inner text",
               },
               vars: Variables{},
               want: "Outer Inner text",
           },
           {
               name:   "skills with variables",
               prompt: "{{skill:template}}",
               skills: map[string]string{"template": "Task {{task_id}}"},
               vars:   Variables{"task_id": "t2"},
               want:   "Task t2",
           },
           {
               name:    "undefined skill",
               prompt:  "{{skill:missing}}",
               skills:  map[string]string{},
               vars:    Variables{},
               wantErr: true,
               errContains: "undefined skill",
           },
           {
               name:   "undefined variable (left as-is)",
               prompt: "{{missing_var}}",
               vars:   Variables{},
               want:   "{{missing_var}}",
           },
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               tmpl := &Template{prompt: tt.prompt, skills: tt.skills}
               got, err := tmpl.Render(tt.vars)
               if (err != nil) != tt.wantErr {
                   t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
                   return
               }
               if tt.wantErr && tt.errContains != "" {
                   if err == nil || !contains(err.Error(), tt.errContains) {
                       t.Errorf("Render() error = %v, want error containing %q", err, tt.errContains)
                   }
               }
               if !tt.wantErr && got != tt.want {
                   t.Errorf("Render() = %q, want %q", got, tt.want)
               }
           })
       }
   }

   func TestCircularSkills(t *testing.T) {
       tmpl := &Template{
           prompt: "{{skill:a}}",
           skills: map[string]string{
               "a": "A {{skill:b}}",
               "b": "B {{skill:a}}",
           },
       }
       _, err := tmpl.Render(Variables{})
       if err == nil {
           t.Error("Render() expected error for circular skills, got nil")
       }
   }

   func contains(s, substr string) bool {
       return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
   }

   func findSubstring(s, substr string) bool {
       for i := 0; i <= len(s)-len(substr); i++ {
           if s[i:i+len(substr)] == substr {
               return true
           }
       }
       return false
   }
   ```

2. **Run tests**
   ```bash
   go test -v ./internal/templates/
   ```

3. **Check coverage**
   ```bash
   go test -cover ./internal/templates/
   ```

## Testing

- [ ] All test cases pass
- [ ] Test coverage > 80%
- [ ] Edge cases covered (missing files, invalid roles, circular refs)
- [ ] No test failures or panics

## Acceptance Criteria

- [ ] `templates_test.go` created with comprehensive tests
- [ ] Tests for Load function (happy path, missing files, invalid roles)
- [ ] Tests for Render function (variables, skills, recursive skills)
- [ ] Test for circular skill detection
- [ ] Test for undefined skill error
- [ ] Test for undefined variable behavior
- [ ] All tests passing: `go test ./internal/templates/`
- [ ] Coverage report shows >80% coverage

## Dependencies

- Depends on: [mol-pte-003](./mol-pte-003.md) - Render function must be implemented

## Notes

- Use `t.TempDir()` for isolated test directories
- Table-driven tests make it easy to add more cases
- Test both happy paths and error conditions
- Consider adding benchmarks if performance becomes a concern
