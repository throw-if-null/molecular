# Implement template Load function

**ID:** mol-pte-002  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Implement the `Load` function in `internal/templates` that reads the main `prompt.md` file and all skill files from `.molecular/<agent>/` directories.

## Context

The Load function is responsible for reading template files from disk. It must:
- Load the mandatory `prompt.md` file (error if missing)
- Load optional skill files from `skills/` subdirectory
- Return a populated Template struct

## Implementation Details

### Files to Modify

- `internal/templates/templates.go` - Replace placeholder `Load` function

### Step-by-Step Instructions

1. **Implement Load function**

   ```go
   import (
       "fmt"
       "os"
       "path/filepath"
       "strings"
   )

   func Load(cfg Config) (*Template, error) {
       // Validate role
       if cfg.Role != "carbon" && cfg.Role != "helium" {
           return nil, fmt.Errorf("invalid role: %s (must be 'carbon' or 'helium')", cfg.Role)
       }

       baseDir := filepath.Join(cfg.RepoRoot, ".molecular", cfg.Role)
       promptPath := filepath.Join(baseDir, "prompt.md")

       // Load main prompt (MANDATORY)
       promptBytes, err := os.ReadFile(promptPath)
       if err != nil {
           if os.IsNotExist(err) {
               return nil, fmt.Errorf("template not found: %s (templates are mandatory for %s)", promptPath, cfg.Role)
           }
           return nil, fmt.Errorf("failed to read template %s: %w", promptPath, err)
       }

       // Load skills (OPTIONAL)
       skills := make(map[string]string)
       skillsDir := filepath.Join(baseDir, "skills")
       entries, err := os.ReadDir(skillsDir)
       if err != nil && !os.IsNotExist(err) {
           return nil, fmt.Errorf("failed to read skills directory %s: %w", skillsDir, err)
       }

       for _, entry := range entries {
           if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
               continue
           }
           skillName := strings.TrimSuffix(entry.Name(), ".md")
           skillPath := filepath.Join(skillsDir, entry.Name())
           skillBytes, err := os.ReadFile(skillPath)
           if err != nil {
               return nil, fmt.Errorf("failed to read skill %s: %w", skillPath, err)
           }
           skills[skillName] = string(skillBytes)
       }

       return &Template{
           prompt: string(promptBytes),
           skills: skills,
       }, nil
   }
   ```

2. **Add helper method for listing skills (optional but useful for debugging)**

   ```go
   // Skills returns the names of all loaded skills.
   func (t *Template) Skills() []string {
       names := make([]string, 0, len(t.skills))
       for name := range t.skills {
           names = append(names, name)
       }
       return names
   }
   ```

3. **Verify the implementation builds**
   ```bash
   go build ./internal/templates/
   ```

## Testing

Testing will be added in mol-pte-004, but manual verification:

```bash
# Create test template structure
mkdir -p /tmp/test-repo/.molecular/carbon/skills
echo "# Test prompt {{task_id}}" > /tmp/test-repo/.molecular/carbon/prompt.md
echo "# Test skill" > /tmp/test-repo/.molecular/carbon/skills/test-skill.md

# Test loading (will be proper unit test later)
go test -v ./internal/templates/
```

## Acceptance Criteria

- [ ] Load function implemented
- [ ] Validates role is "carbon" or "helium"
- [ ] Returns error if prompt.md is missing
- [ ] Loads main prompt.md correctly
- [ ] Loads all .md files from skills/ directory
- [ ] Ignores non-.md files and directories in skills/
- [ ] Returns populated Template struct with prompt and skills
- [ ] Handles missing skills/ directory gracefully (no error)
- [ ] Package builds without errors

## Dependencies

- Depends on: [mol-pte-001](./mol-pte-001.md) - Package structure must exist

## Notes

- Use `os.ReadFile` for simple file reading (no need for streaming)
- Skill names are derived from filenames (e.g., `checklist.md` → `checklist`)
- Error messages should be clear and actionable
- Missing skills/ directory is not an error (skills are optional)
