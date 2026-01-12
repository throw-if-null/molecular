---
description: "Go implementation subagent: writes Go code + tests, runs go test, minimal diffs."
model: "github-copilot/gpt-5-mini"
mode: subagent
reasoningEffort: medium
temperature: 0.2
prompt: prompts/gopher.prompt.md
permission:
  edit:
    "*.go": allow
    "go.mod": allow
    "go.sum": allow
    "*": deny
  read:
    "*": allow
    "*.env": deny
    "*.env.*": deny
    ".env": deny
    ".env.*": deny
  bash: allow
  webfetch: deny
  websearch: deny
  codesearch: allow
  skill:
    "*": deny
  external_directory: deny
---
