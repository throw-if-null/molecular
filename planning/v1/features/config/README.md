# Feature: Configuration

## Goal

Define and implement a stable configuration mechanism so Molecular can be run with sane defaults, but can be customized per repository.

## Current state

- `molecular doctor` checks for `.molecular/config.toml` presence.
- There is no real config parsing or wiring; most values are hard-coded defaults.

## Proposed schema (v1)

File: `.molecular/config.toml`

Suggested keys (example):

```toml
[silicon]
# bind = "127.0.0.1:8711"  # optional, keep default
poll_interval_ms = 50

[retry]
carbon_budget = 3
helium_budget = 3
review_budget = 2

[workers]
carbon_concurrency = 1
helium_concurrency = 1

[hooks]
# enabled = true
# lithium = ".molecular/lithium.sh"
# chlorine = ".molecular/chlorine.sh"
```

## Detailed implementation steps

1. Decide file format
   - Option A: keep TOML and add a TOML parser dependency.
   - Option B: switch to JSON (`config.json`) to avoid dependencies.
   - Step: choose one and document it in `README.md`.

2. Create config package
   - Add `internal/config` with:
     - `type Config struct { ... }`
     - defaults constructor: `Default()`
     - loader: `Load(repoRoot string) (Config, error)`
   - Loader should:
     - start from defaults
     - if config file missing: return defaults (no error)
     - if present but invalid: return a typed error (so doctor/status can surface it)

3. Define discovery rules
   - First iteration: only repo-local `.molecular/config.*`.
   - Later: allow env/flag override.

4. Wire into Silicon
   - Silicon startup should call config loader.
   - Pass config values into worker loop parameters:
     - poll interval
     - retry budgets defaults for new tasks
     - concurrency (when implemented)

5. Wire into task creation
   - `POST /v1/tasks` should apply default budgets from config when creating a new task.
   - Consider whether budgets become immutable per task after creation (recommended).

6. Update doctor
   - Doctor should validate config by actually parsing it.
   - `--json` output should include validity + parse error summary.

7. Tests
   - Unit tests in `internal/config`:
     - missing config => defaults
     - invalid config => error
     - partial config => merges with defaults
   - Integration-ish tests for server start with config.

## Acceptance criteria

- Defaults work without config.
- Config file can set retry budgets/poll interval.
- Doctor reports invalid config clearly.
- No config changes are required for "hello world" usage.
