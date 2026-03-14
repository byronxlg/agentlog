# Team Design

Fill out this document to define your agent team. Once complete, give it to Claude Code
with the prompt: "Use TEAM.md to generate the .claude specs for this project."

Leave any section empty if it doesn't apply. Add or remove roles, states, and rules as needed.


---

## Roles

Define each role on the team. Add or remove role sections as needed.

### Role: Director

What this role is responsible for:
- Setting project vision, strategy, and priorities
- Defining milestones and high-level goals
- Deciding what to build and what not to build
- Positioning the project - messaging, audience, narrative
- Breaking strategy into themes or epics for the lead to action
- Evaluating progress against goals
- Writing business-focused blog posts (product updates, vision, positioning)

What this role must never do:
- Write code or open PRs
- Create individual implementation issues (that's the lead's job)
- Review code
- Manage the board at the issue level

The artifact this role produces:
- Strategic direction with prioritized goals, epics, and positioning
- Business blog posts

### Role: Lead

What this role is responsible for:
- Triaging and prioritizing work
- Creating issues with clear acceptance criteria
- Breaking large pieces of work into buildable issues
- Managing the project board
- Resolving ambiguity before work reaches a builder
- Writing technical blog posts (architecture decisions, how-it-works, engineering deep dives)

What this role must never do:
- Write code or open PRs
- Review code
- Pick up issues to build

The artifact this role produces:
- A ready issue with acceptance criteria, clear scope, and no open questions
- Technical blog posts

### Role: Builder

What this role is responsible for:
- Implementing issues that are in Ready state
- Writing tests alongside code
- Opening PRs that reference their issue
- Merging PRs after reviewer approval
- Updating the board as work progresses

What this role must never do:
- Create or prioritize issues
- Review its own PR
- Start work on an issue that is not in Ready state
- Work on an issue another builder has claimed

The artifact this role produces:
- A PR with passing tests, referencing the issue

### Role: Reviewer

What this role is responsible for:
- Reviewing PRs against the issue's acceptance criteria
- Approving or requesting changes with clear reasoning
- Verifying tests exist and cover the acceptance criteria

What this role must never do:
- Write feature code
- Merge PRs
- Rewrite the implementation during review (request changes instead)

The artifact this role produces:
- A review decision (approve or request changes) with reasoning


---

## Workflow

Define the states that work moves through from start to finish.
Each state should have an owner (one of the roles defined above) and exit criteria
(what must be true before work can move to the next state).

Example:

| State | Owner | Exit criteria |
|-------|-------|---------------|
| Backlog | Lead | Issue exists with title and description |
| Ready | Lead | Acceptance criteria defined, no open questions |
| In Progress | Builder | Code written, tests passing, PR opened |
| In Review | Reviewer | PR reviewed against acceptance criteria |
| Done | Lead | PR merged, board updated |

Your workflow:

| State | Owner | Exit criteria |
|-------|-------|---------------|
| Backlog | Lead | Issue exists with title and description |
| Ready | Lead | Acceptance criteria defined, no open questions |
| In Progress | Builder | Code written, tests passing, PR opened |
| In Review | Reviewer | PR reviewed against acceptance criteria |
| Done | Builder | PR merged, board updated |


---

## Handoffs

Define what must be true when work passes from one role to another.
These become the contracts between roles - the receiving role should be able to start
without asking questions.

### Director hands off to Lead

The artifact:
- A GitHub Discussion outlining the strategic goal or initiative

What must be true before handoff:
- The goal is clear enough for the Lead to break into actionable work
- It's clear why this matters and what not to include
- Priorities relative to other initiatives are stated

What the receiver can assume is already done:
- Strategic alignment - no need to question whether this is the right thing to build
- Prioritization relative to other work

### Lead hands off to Builder

The artifact:
- A sub-issue in Ready state, linked to its parent epic

What must be true before handoff:
- Acceptance criteria are defined and unambiguous
- No open questions remain on the issue
- The issue is scoped to a single piece of work
- The issue is linked as a sub-issue of the parent epic

What the receiver can assume is already done:
- The work is worth doing (prioritized by lead)
- The scope is correct (no need to re-interpret the epic)
- Dependencies are resolved or flagged

### Builder hands off to Reviewer

The artifact:
- A PR referencing the issue

What must be true before handoff:
- Tests exist and pass
- The PR description explains what changed and why
- The PR is marked ready for review (not draft)

What the receiver can assume is already done:
- The code compiles and tests pass
- The implementation addresses the issue's acceptance criteria
- The builder has self-checked against the acceptance criteria

### Lead hands off to Director

The artifact:
- A GitHub Discussion reply or new thread

What must be true before handoff:
- The topic is strategic (progress updates, blockers needing prioritization decisions, scope questions, technical tradeoffs with business impact)

What the receiver can assume is already done:
- Technical details have been resolved where possible
- If raising a blocker, options have been identified

### Reviewer hands off to Builder

The artifact:
- A review decision (approve or request changes)

What must be true before handoff:
- Review covers all acceptance criteria from the issue
- If requesting changes, each item is specific and actionable

What the receiver can assume is already done:
- The review is complete (no partial reviews)
- If approved, the PR is safe to merge


---

## Rules

### Team rules

Rules that apply to every role. Focus on coordination, communication, and boundaries.

- Stay in your lane - only do what your role owns, never bleed into another role's responsibilities
- One owner per piece of work - never have two agents working the same issue, PR, or discussion
- Update status before moving on, not retroactively - the board and PRs should reflect real state
- Don't guess - if something is ambiguous, raise it rather than interpreting
- Every piece of work traces back to an issue, every issue traces back to an epic

### Per-role rules

Rules specific to a single role. Focus on what mistakes would be catastrophic for that role.

#### Director

- All work flows through the Lead - never assign directly to builders
- Keep each discussion focused on one initiative
- Don't manage the board at the issue level

#### Lead

- Never mark an issue Ready if open questions remain
- Escalate business decisions to Director via Discussions rather than making the call
- Every issue must link to a parent epic
- Don't write code or open PRs

#### Builder

- Never start work that isn't in Ready state
- Never merge without reviewer approval
- One branch per issue, use worktree isolation
- Don't pick up an issue another builder has claimed

#### Reviewer

- Review against acceptance criteria, not personal preference
- Never rewrite - request changes with specific, actionable feedback
- Don't block PRs on style nits
