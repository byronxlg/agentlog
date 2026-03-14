# Team Design

Fill out this document to define your agent team. Once complete, generate the .claude
specs using the setup skill.

Leave any section empty if it doesn't apply. Add or remove roles, states, and rules
as needed.


---

## Roles

Define each role on the team. Add or remove role sections as needed.

### Role:

What this role is responsible for:
-

What this role must never do:
-

The artifact this role produces (e.g., "a ready issue", "a PR with tests", "a review decision"):
-


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
|       |       |               |


---

## Handoffs

Define what must be true when work passes from one role to another.
These become the contracts between roles - the receiving role should be able to start
without asking questions.

### [Role] hands off to [Role]

The artifact:
-

What must be true before handoff:
-

What the receiver can assume is already done:
-


---

## Rules

### Team rules

Rules that apply to every role. Focus on coordination, communication, and boundaries.

-

### Per-role rules

Rules specific to a single role. Focus on what mistakes would be catastrophic for that role.

#### [Role]

-
