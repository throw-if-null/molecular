---
description: "Implementation agent: writes code, runs tests, and iterates in an isolated worktree."
model: "github-copilot/gpt-5-mini"
reasoningEffort: high
verbosity: high
temperature: 0.4
prompt: prompts/builder.prompt.md
permission:
  edit: allow
  bash: allow
  webfetch: allow
  websearch: allow
  codesearch: allow
  skill:
    "*": deny
    greeter: allow
    builder-checklist: allow
    builder-signoff: allow
external_directory: deny
---
