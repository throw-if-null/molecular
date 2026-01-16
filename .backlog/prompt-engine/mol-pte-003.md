# Implement template Render function

**ID:** mol-pte-003  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Implement the `Render` function that performs variable substitution and recursive skill composition in loaded templates.

## Context

The Render function takes a loaded Template and a Variables map, then:
1. Recursively replaces skill references (`{{skill:name}}`) with skill content
2. Replaces variable references (`{{variable}}`) with provided values
3. Detects and reports circular skill dependencies
4. Returns the final rendered string

## Implementation Details

### Files to Modify

- `internal/templates/templates.go` - Replace placeholder `Render` function

### Step-by-Step Instructions

1. **Add regex patterns at package level**

   ```go
   import (
       "regexp"
   )

   var (
       varRegex   = regexp.MustCompile(`\{\{([a-z_]+)\}\}`)
       skillRegex = regexp.MustCompile(`\{\{skill:([a-z-]+)\}\}`)
   )
   ```

2. **Implement Render function**

   ```go
   func (t *Template) Render(vars Variables) (string, error) {
       rendered := t.prompt

       // First pass: replace skills (supports recursive skills)
       visited := make(map[string]bool)
       maxIterations := 100 // Prevent infinite loops
       iteration := 0

       for {
           matches := skillRegex.FindAllStringSubmatch(rendered, -1)
           if len(matches) == 0 {
               break
           }
           
           iteration++
           if iteration > maxIterations {
               return "", fmt.Errorf("skill rendering exceeded max iterations (possible circular reference)")
           }

           replaced := false
           for _, match := range matches {
               fullMatch := match[0]  // e.g., "{{skill:checklist}}"
               skillName := match[1]  // e.g., "checklist"

               if visited[skillName] {
                   continue // Already replaced this skill in current iteration
               }

               skillContent, ok := t.skills[skillName]
               if !ok {
                   return "", fmt.Errorf("undefined skill: %s", skillName)
               }

               rendered = strings.ReplaceAll(rendered, fullMatch, skillContent)
               visited[skillName] = true
               replaced = true
           }

           if !replaced {
               break
           }
       }

       // Second pass: replace variables
       rendered = varRegex.ReplaceAllStringFunc(rendered, func(match string) string {
           submatches := varRegex.FindStringSubmatch(match)
           if len(submatches) < 2 {
               return match
           }
           varName := submatches[1]

           if val, ok := vars[varName]; ok {
               return val
           }

           // Leave undefined variables as-is (could make this an error if desired)
           return match
       })

       return rendered, nil
   }
   ```

3. **Add optional helper to validate template before rendering**

   ```go
   // ValidateSkills checks that all skill references can be resolved.
   // Returns a list of undefined skill names, or empty slice if all are valid.
   func (t *Template) ValidateSkills() []string {
       var undefined []string
       matches := skillRegex.FindAllStringSubmatch(t.prompt, -1)
       seen := make(map[string]bool)

       for _, match := range matches {
           skillName := match[1]
           if seen[skillName] {
               continue
           }
           seen[skillName] = true

           if _, ok := t.skills[skillName]; !ok {
               undefined = append(undefined, skillName)
           }
       }

       return undefined
   }
   ```

4. **Verify the implementation builds**
   ```bash
   go build ./internal/templates/
   ```

## Testing

Testing will be comprehensive in mol-pte-004, but manual verification:

```go
// Test rendering
tmpl := &Template{
    prompt: "# {{task_id}}\n{{skill:test}}",
    skills: map[string]string{"test": "Skill content"},
}
result, err := tmpl.Render(Variables{"task_id": "test-123"})
// Should produce: "# test-123\nSkill content"
```

## Acceptance Criteria

- [ ] Render function implemented
- [ ] Variable substitution works (`{{var}}` → value)
- [ ] Skill substitution works (`{{skill:name}}` → skill content)
- [ ] Recursive skill rendering works (skills can reference other skills)
- [ ] Circular skill references are detected (returns error)
- [ ] Undefined skills return clear error
- [ ] Undefined variables are left as-is (or error if strict mode desired)
- [ ] Package builds without errors
- [ ] No panics on malformed input

## Dependencies

- Depends on: [mol-pte-002](./mol-pte-002.md) - Load function must exist

## Notes

- Skill rendering happens first, then variable substitution
- This allows skills to contain variable references
- The `visited` map prevents infinite loops during skill resolution
- Consider making undefined variables an error in future (currently they're left as-is)
- Regex patterns match lowercase alphanumeric + underscore/hyphen only
