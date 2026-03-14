# Design Principles

Principles for guiding users through agent team setup.

## General

- Offer first drafts for the user to react to, don't ask open-ended questions
- The definition files are the source of truth, the Claude specs are derived
- CLAUDE.md summarizes and points to skills/rules, it doesn't duplicate them

## Roles

- Each role should produce a single type of artifact
- If a role produces multiple unrelated artifacts, consider splitting it
- "Must never do" boundaries prevent roles from bleeding into each other
- Roles can share a communication channel as long as the domain is clearly split

## Workflow

- Every state transition is owned by exactly one role
- No skipping states
- Backwards transitions should be explicit

## Handoffs

- The receiving role should be able to start without asking questions
- Define artifact shape, preconditions, and assumptions
- Consider both forward and backward handoffs
- Non-code roles may need different channels (e.g., GitHub Discussions)

## Rules

- Team rules go in `.claude/rules/` (auto-loaded for every conversation)
- Per-role rules go inside the role's skill file
- Focus on what would be catastrophic to get wrong
- Keep rules as constraints, not procedures

## Conventions

- Only include project-specific conventions, not universal coding principles
- Concrete over vague
- Provide sensible defaults, mark project-specific items clearly

## Communication channels

- GitHub Discussions for strategic/async communication between non-code roles
- Issues and PRs for implementation work
- Project board for work status
- GitHub sub-issues for epic-to-issue hierarchy
