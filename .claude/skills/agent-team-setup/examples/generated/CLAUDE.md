# Agent Team

A framework for creating and managing teams of Claude Code agents that work on projects independently, with defined roles, workflows, and coordination rules.

**Tech stack:** Claude Code, GitHub (Issues, Discussions, Projects), Markdown

## Team

| Role | Owns |
|------|------|
| Director | Vision, strategy, priorities, business blog posts |
| Lead | Issue creation, project board, technical blog posts |
| Builder | Implementation, PRs, merging after approval |
| Reviewer | Code review, approve/request changes |

## Workflow

| State | Owner | Exit criteria |
|-------|-------|---------------|
| Backlog | Lead | Issue exists with title and description |
| Ready | Lead | Acceptance criteria defined, no open questions |
| In Progress | Builder | Code written, tests passing, PR opened |
| In Review | Reviewer | PR reviewed against acceptance criteria |
| Done | Builder | PR merged, board updated |

## Communication

- **Director <-> Lead:** GitHub Discussions
- **Lead -> Builder:** GitHub Issues (sub-issues linked to epics)
- **Builder <-> Reviewer:** Pull Requests
- **Status tracking:** GitHub Projects board

## Skills

- `/agent-team-setup` - set up a new agent team project from scratch
- `/director` - act as the Director role
- `/lead` - act as the Lead role
- `/builder` - act as the Builder role
- `/reviewer` - act as the Reviewer role

## Conventions

Development conventions are in `.claude/rules/` and are auto-loaded into every conversation.

## Source of truth

Team design, project details, and conventions are defined in:
- `PROJECT.md` - project identity
- `TEAM.md` - roles, workflow, handoffs, rules
- `CONVENTIONS.md` - development conventions
