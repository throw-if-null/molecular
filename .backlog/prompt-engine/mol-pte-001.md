# Create internal/templates package structure

**ID:** mol-pte-001  
**Status:** todo  
**Feature:** [Prompt Template Engine](./index.md)

## Objective

Create the foundational `internal/templates` package with proper Go module structure, exported types, and placeholder functions for template loading and rendering.

## Context

This is the first task in the prompt template engine feature. It establishes the package structure that subsequent tasks will build upon. The package needs to provide a clean API for loading templates from `.molecular/<agent>/` directories and rendering them with variables.

## Implementation Details

### Files to Create

- `internal/templates/templates.go` - Main package file with types and functions
- `internal/templates/doc.go` - Package documentation

### Step-by-Step Instructions

1. **Create package directory**
   ```bash
   mkdir -p internal/templates
   ```

2. **Create doc.go with package documentation**
   ```go
   // Package templates provides template loading and rendering for Carbon and Helium agents.
   //
   // Templates are loaded from .molecular/<agent>/ directories and support:
   // - Variable substitution: {{variable_name}}
   // - Skill composition: {{skill:skill-name}}
   //
   // Example directory structure:
   //   .molecular/carbon/prompt.md       (main template, mandatory)
   //   .molecular/carbon/skills/*.md     (reusable snippets, optional)
   package templates
   ```

3. **Create templates.go with type definitions**
   
   Define these types:
   
   ```go
   package templates

   import (
       "fmt"
   )

   // Config specifies the template loading configuration.
   type Config struct {
       RepoRoot string // Repository root path
       Role     string // Agent role: "carbon" or "helium"
   }

   // Variables holds template variable values for substitution.
   type Variables map[string]string

   // Template represents a loaded prompt template with skills.
   type Template struct {
       prompt string            // Main prompt content
       skills map[string]string // Skill name -> content mapping
   }
   ```

4. **Add placeholder functions**
   
   Add these function signatures (implementation in later tasks):
   
   ```go
   // Load reads the main prompt and all skills for a given role.
   // Returns an error if the main prompt.md file is not found.
   func Load(cfg Config) (*Template, error) {
       return nil, fmt.Errorf("not implemented")
   }

   // Render replaces variables and skills in the template.
   // Returns an error if undefined variables/skills are referenced or circular dependencies exist.
   func (t *Template) Render(vars Variables) (string, error) {
       return "", fmt.Errorf("not implemented")
   }
   ```

5. **Verify package builds**
   ```bash
   go build ./internal/templates/
   ```

## Testing

- [ ] Package compiles successfully
- [ ] `go build ./internal/templates/` succeeds
- [ ] Package documentation is clear (readable via `go doc`)

## Acceptance Criteria

- [ ] `internal/templates/` directory created
- [ ] `doc.go` exists with package documentation
- [ ] `templates.go` exists with Config, Variables, Template types
- [ ] Placeholder `Load` and `Render` functions defined
- [ ] Package builds without errors
- [ ] No external dependencies added (stdlib only for now)

## Dependencies

None - this is the foundational task.

## Notes

- Keep the API surface minimal initially
- Focus on clear type definitions and documentation
- Implementation of Load and Render will come in mol-pte-002 and mol-pte-003
