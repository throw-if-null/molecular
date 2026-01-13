# Feature: Bash Completions for `molecular` CLI

## Goal

Add bash completion support for the `molecular` CLI to improve developer experience with tab-completion for commands, flags, and task IDs.

## Current state

- No shell completions available
- Users must type full commands and task IDs manually
- No discoverability of available commands

## Requirements

### Completion support

- Commands: `submit`, `status`, `list`, `cancel`, `logs`, `cleanup`, `history`, `doctor`, `version`
- Flags: `--task-id`, `--prompt`, `--json`, `--tail`, `--watch`, `--interval`, `--limit`
- Dynamic task ID completion (from `molecular list`)

### Installation

User installs completions once:

```bash
# System-wide installation (requires sudo)
sudo cp scripts/completions/bash/molecular /etc/bash_completion.d/

# User installation (no sudo required)
mkdir -p ~/.local/share/bash-completion/completions
cp scripts/completions/bash/molecular ~/.local/share/bash-completion/completions/

# Or source directly in ~/.bashrc
echo 'source /path/to/molecular/scripts/completions/bash/molecular' >> ~/.bashrc
```

### Completion behavior

**Command completion:**
```bash
$ molecular <TAB>
submit  status  list  cancel  logs  cleanup  history  doctor  version

$ molecular st<TAB>
status
```

**Flag completion:**
```bash
$ molecular status --<TAB>
--json  --watch

$ molecular logs --<TAB>
--json  --tail  --attempt-id
```

**Task ID completion:**
```bash
$ molecular status <TAB>
feat-123  bug-456  refactor-789

$ molecular status fe<TAB>
feat-123
```

## Detailed implementation steps

### 1. Create completion script

**File:** `scripts/completions/bash/molecular`

**Implementation:**
```bash
#!/usr/bin/env bash

_molecular_completions() {
    local cur prev words cword
    _init_completion || return

    local commands="submit status list cancel logs cleanup history doctor version"
    local global_flags="--help --version"

    # Complete commands
    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return 0
    fi

    local command="${words[1]}"

    case "$command" in
        submit)
            local flags="--task-id --prompt"
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "$flags" -- "$cur"))
            fi
            ;;
        status|cancel|logs|cleanup|history)
            local flags="--json"
            if [[ $command == "logs" ]]; then
                flags="$flags --tail --attempt-id"
            fi
            if [[ $command == "status" ]]; then
                flags="$flags --watch --interval"
            fi
            
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "$flags" -- "$cur"))
            elif [[ $cword -eq 2 ]]; then
                # Complete task IDs
                local task_ids=$(molecular list --format=ids 2>/dev/null)
                COMPREPLY=($(compgen -W "$task_ids" -- "$cur"))
            fi
            ;;
        list)
            local flags="--limit --json"
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "$flags" -- "$cur"))
            fi
            ;;
        doctor)
            local flags="--json"
            if [[ $cur == -* ]]; then
                COMPREPLY=($(compgen -W "$flags" -- "$cur"))
            fi
            ;;
    esac
}

complete -F _molecular_completions molecular
```

### 2. Add `--format=ids` to `list` command

**File:** `cmd/molecular/main.go`

**Changes:**
- Add `--format=ids` flag to `list` command
- Output only task IDs (one per line) when flag is set
- Used by completion script for dynamic task ID completion

**Example:**
```bash
$ molecular list --format=ids
feat-123
bug-456
refactor-789
```

### 3. Create installation instructions

**File:** `scripts/completions/README.md`

**Content:**
```markdown
# Shell Completions

## Bash

### Installation

**System-wide (requires sudo):**
\```bash
sudo cp bash/molecular /etc/bash_completion.d/
\```

**User-local:**
\```bash
mkdir -p ~/.local/share/bash-completion/completions
cp bash/molecular ~/.local/share/bash-completion/completions/
\```

**Manual source in ~/.bashrc:**
\```bash
source /path/to/molecular/scripts/completions/bash/molecular
\```

### Usage

After installation, restart your shell or run:
\```bash
source ~/.bashrc
\```

Then use tab completion:
\```bash
molecular st<TAB>       # Completes to 'status'
molecular status <TAB>  # Lists available task IDs
\```

## Zsh, Fish, PowerShell

Future work. Contributions welcome!
```

### 4. Update main README

**File:** `README.md`

**Add section after "Install / build":**
```markdown
## Shell Completions

Bash completions are available for command and task ID completion.

Install:
\```bash
sudo cp scripts/completions/bash/molecular /etc/bash_completion.d/
# Then restart your shell
\```

See `scripts/completions/README.md` for details.
```

### 5. Tests

**Manual testing:**
- Source completion script
- Verify command completion works
- Verify flag completion works
- Verify task ID completion works (requires running Silicon + submitted tasks)

**Automated testing (optional):**
- Bash completion testing is notoriously difficult to automate
- Focus on ensuring `molecular list --format=ids` works correctly (this can be unit tested)

## Acceptance criteria

- [ ] Completion script created in `scripts/completions/bash/molecular`
- [ ] Command completion works
- [ ] Flag completion works per command
- [ ] Task ID completion works (dynamic via `list --format=ids`)
- [ ] Installation instructions documented
- [ ] `molecular list --format=ids` implemented
- [ ] README updated with completion installation

## Example usage

```bash
# After installation and sourcing completions:

$ molecular <TAB>
submit  status  list  cancel  logs  cleanup  history  doctor  version

$ molecular status <TAB>
feat-123  bug-456  refactor-789

$ molecular status feat-123 --<TAB>
--json  --watch

$ molecular logs feat-123 --<TAB>
--json  --tail  --attempt-id
```

## Follow-up work (post-v1)

- Zsh completions
- Fish completions
- PowerShell completions (Windows support)
- Attempt ID completion for `--attempt-id` flag
- Role completion for future `--role` filters
