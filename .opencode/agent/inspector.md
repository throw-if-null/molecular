---
description: "Review agent: gates changes, enforces standards, and provides feedback."
model: "github-copilot/claude-haiku-4.5" #"github-copilot/gpt-5.1"
temperature: 0.1
prompt: prompts/inspector.prompt.md
permission:
  edit:
    "*": deny
    inspector_result.json: allow
  read:
    "*": allow
    "*.env": deny
    "*.env.*": deny
    ".env": deny
    ".env.*": deny
  bash: allow
  webfetch: allow
  websearch: allow
  codesearch: allow
  skill:
    "*": deny
    greeter: allow
    inspector-checklist: allow
    inspector-signoff: allow
  external_directory: deny
---
