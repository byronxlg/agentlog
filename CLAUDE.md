# agentlog

A local-first, framework-agnostic decision log daemon for agentic workflows. Captures agent decisions and reasoning - the missing layer between git history and LLM traces.

**Tech stack:** Go (daemon), SQLite (index), JSONL (log format), Python SDK, TypeScript SDK, TOML (config)

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

- **Director <-> Lead:** GitHub Discussions (Initiatives category)
- **Builder / Reviewer -> Lead:** GitHub Discussions (Engineering category)
- **Human -> Team:** GitHub Discussions (Human category)
- **Blog posts:** GitHub Discussions (Public category)
- **Status updates:** GitHub Discussions (Status Updates category) - all roles post every run, Lead reads before starting
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
