# Feature: Prompt Templates + Skills System

## Goal

Implement a mandatory prompt template system that allows users to define custom instructions, procedures, and skills for Carbon (builder) and Helium (inspector) agents. This ports and improves upon the `.foreman/` template system.

## Current state

- Carbon and Helium workers execute commands with only the user's task prompt
- No mechanism for injecting repository-specific instructions, checklists, or constraints
- `.foreman/` directory contains legacy template system with skills support

## Requirements

### Mandatory templates

- Templates are **required** for Carbon and Helium to function
- Users **must** provide custom templates in `.molecular/<agent>/` directories
- `molecular doctor` should validate template presence and report missing templates
- Silicon workers should fail gracefully if templates are missing

### Directory structure

```
.molecular/
├── carbon/
│   ├── prompt.md              # Main Carbon prompt template (required)
│   └── skills/                # Optional skills directory
│       ├── checklist.md       # Implementation checklist skill
│       └── conventions.md     # Coding conventions skill
├── helium/
│   ├── prompt.md              # Main Helium prompt template (required)
│   └── skills/                # Optional skills directory
│       └── review-rules.md    # Review guidelines skill
└── config.toml
```

### Template format

Templates are Markdown files with:
1. **Fixed instructions** (agent role, procedures, constraints)
2. **Variable substitution** using `{{variable}}` syntax
3. **Skill references** using `{{skill:skill-name}}` syntax

Available variables:
- `{{task_id}}`: unique task identifier
- `{{task_prompt}}`: user's original task prompt
- `{{role}}`: agent role (carbon/helium)
- `{{repo_root}}`: absolute path to repository root
- `{{worktree_path}}`: absolute path to task worktree
- `{{artifacts_root}}`: absolute path to artifacts directory

### Skill system

Skills are reusable Markdown snippets stored in `.molecular/<agent>/skills/`:
- Filename determines skill name: `checklist.md` → `{{skill:checklist}}`
- Skills can reference other skills (recursive rendering)
- Skills can use template variables
- Skills are rendered inline when referenced

### Template rendering

When Carbon/Helium starts an attempt:
1. Load `<agent>/prompt.md` template
2. Load all skills from `<agent>/skills/*.md`
3. Render variables and skill references recursively
4. Prepend rendered template to user's task prompt
5. Pass final prompt to agent command via stdin or file

## Detailed implementation steps

### 1. Create `internal/templates` package

**Files:**
- `internal/templates/templates.go`: core loading and rendering logic
- `internal/templates/templates_test.go`: unit tests

**API:**
```go
package templates

type Config struct {
    RepoRoot string
    Role     string // "carbon" or "helium"
}

type Variables map[string]string

// Load loads and validates templates for a given role
func Load(cfg Config) (*Template, error)

// Render renders a template with variables and skills
func (t *Template) Render(vars Variables) (string, error)
```

**Responsibilities:**
- Load `<role>/prompt.md` from `.molecular/`
- Load all skills from `<role>/skills/*.md`
- Validate template syntax (report undefined variables/skills)
- Render variables: `{{var}}` → `vars["var"]`
- Render skills: `{{skill:name}}` → inline skill content
- Handle recursive skill references (detect cycles)
- Return rendered prompt as string

### 2. Update Carbon worker

**File:** `internal/silicon/worker.go` (Carbon section)

**Changes:**
- Load template during attempt setup
- Build variables map from task metadata
- Render template to string
- Prepend rendered template to user prompt
- Pass final prompt to Carbon command

**Variable binding:**
```go
vars := templates.Variables{
    "task_id":        t.TaskID,
    "task_prompt":    t.Prompt,
    "role":           "carbon",
    "repo_root":      repoRoot,
    "worktree_path":  t.WorktreePath,
    "artifacts_root": t.ArtifactsRoot,
}
```

**Error handling:**
- If template load fails: mark attempt failed with clear error
- If render fails: mark attempt failed with clear error
- Log template errors to attempt `log.txt`

### 3. Update Helium worker

**File:** `internal/silicon/worker.go` (Helium section)

**Same changes as Carbon:**
- Load `helium/prompt.md` template
- Render with Helium-specific variables
- Prepend to user prompt
- Pass to Helium command

### 4. Update `molecular doctor`

**File:** `cmd/molecular/main.go` (doctor command)

**New checks:**
- `.molecular/carbon/prompt.md` exists and readable
- `.molecular/helium/prompt.md` exists and readable
- Skills directories exist (optional, no error if missing)
- Template syntax validation (attempt to load + render with dummy vars)

**Output:**
```
✓ .molecular/carbon/prompt.md found
✓ .molecular/helium/prompt.md found
✓ .molecular/carbon/skills/ found (2 skills)
✓ .molecular/helium/skills/ found (1 skill)
✓ Templates valid
```

### 5. Migrate `.foreman/` templates

**Files to port:**
- `.foreman/builder/templates/builder.template.md` → `.molecular/carbon/prompt.md`
- `.foreman/inspector/templates/inspector.template.md` → `.molecular/helium/prompt.md`

**Migration steps:**
- Extract fixed instructions from `.foreman/` templates
- Convert to `{{variable}}` syntax
- Create initial skills (checklist, review-rules)
- Update README with template usage guide

### 6. Update documentation

**Files:**
- `README.md`: Add "Templates" section
- `planning/v1/features/templates/TEMPLATE_GUIDE.md`: Detailed template authoring guide

**README section:**
```markdown
## Templates

Molecular requires custom prompt templates for Carbon and Helium agents.

Templates live in `.molecular/<agent>/prompt.md` and define:
- Agent role and procedures
- Repository-specific constraints
- Skill references for reusable instructions

See `planning/v1/features/templates/TEMPLATE_GUIDE.md` for authoring guide.
```

### 7. Tests

**Unit tests (`internal/templates/templates_test.go`):**
- Load valid template
- Load missing template (error)
- Render variables
- Render skills
- Detect skill cycles
- Undefined variable handling
- Undefined skill handling

**Integration tests:**
- Carbon worker with template
- Helium worker with template
- Template load failure handling
- `molecular doctor` template validation

## Acceptance criteria

- [ ] Templates are mandatory (workers fail gracefully without them)
- [ ] `molecular doctor` validates templates
- [ ] Carbon worker renders templates before execution
- [ ] Helium worker renders templates before execution
- [ ] Skills system supports recursive references
- [ ] Documentation includes template authoring guide
- [ ] `.foreman/` templates migrated to `.molecular/`
- [ ] All tests passing

## Example templates

### `.molecular/carbon/prompt.md`

```markdown
# Carbon Agent

You are **Carbon**, the implementation agent for this repository.

## Task

{{task_prompt}}

## Environment

- Task ID: `{{task_id}}`
- Worktree: `{{worktree_path}}`
- Artifacts: `{{artifacts_root}}`

## Implementation Checklist

{{skill:checklist}}

## Final Handoff

When complete, write `builder_result.json` following this contract:

{{skill:handoff-format}}
```

### `.molecular/carbon/skills/checklist.md`

```markdown
1. Read AGENTS.md and REVIEW_RULEBOOK.md if present
2. Implement required changes with minimal diffs
3. Run tests and validation commands
4. Stage changes with git add
5. Create commit with clear message
6. Execute final handoff procedure
```

## Migration notes

The `.foreman/` template system used Python string formatting and skill tool calls. Molecular's template system is simpler:
- Pure file-based (no tool calls)
- Markdown-native (no Python string escaping)
- Static rendering at worker start (not dynamic during agent execution)

This trade-off prioritizes simplicity and debuggability over dynamic skill loading.

## Follow-up work (post-v1)

- Template hot-reload during development
- Template validation CLI command
- Template examples/starter kits for common project types
- Schema validation for template variables
