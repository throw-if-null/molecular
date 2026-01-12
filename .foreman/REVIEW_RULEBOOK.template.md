# REVIEW_RULEBOOK (template)

This file defines project-specific rules for Builder and Inspector.

Consumers of the Foreman template should copy/rename this file to
`REVIEW_RULEBOOK.md` and fill in the sections below for their repo.

## 1. Project Overview
- High-level description of the project.
- Primary tech stack (frameworks, languages, runtime targets).
- Any important architectural notes (monorepo, packages, apps vs. libs).

## 2. Tech Stack & Runtime Constraints
- Supported runtime environments (browsers, Node versions, SSR requirements).
- Any restrictions on top-level `await`, dynamic imports, or bundling.
- Guidelines around side effects (e.g., no module-level DOM access).

## 3. Coding Rules & Conventions
- Preferred patterns for components, state management, and data flow.
- File/folder organization rules (where to put new code).
- Style decisions (naming, props/events/snippets, error handling).
- Any framework-specific rules (e.g., how to use Svelte runes, React hooks, etc.).

## 4. Public API & Stability
- What counts as public API in this repo (exports, routes, components,
  CSS hooks, configuration, etc.).
- Stability expectations and versioning model (semver, breaking change policy).
- Expectations around documenting new/changed public behavior.

## 5. Testing Requirements
- Required commands to run for code changes (e.g. `lint`, `check`, `test`,
  `prepack`), and when each is mandatory.
- When to add or update unit tests vs. integration/E2E tests.
- Any special coverage or determinism requirements.

## 6. Accessibility & UX
- Expectations for interactive components (keyboard support, focus,
  semantics, ARIA usage, contrast, motion preferences).
- Any additional UX guidelines relevant to this project.

## 7. Non-goals / Anti-patterns
- Things contributors should generally avoid (e.g., broad refactors,
  new dependencies, global styling changes) unless explicitly requested.
- Known anti-patterns that should trigger review comments.

## 8. Repo-specific Notes (optional)
- Any other constraints or conventions that do not fit above but are
  important for Builder and Inspector to enforce.
