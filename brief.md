# agentlog — Project Brief

> _git log, but for why your agent did things_

---

## The Problem

Every agentic workflow has the same blind spot: you know **what** happened, but not **why**.

Existing observability tools (Langfuse, LangSmith, Helicone) are trace-first. They capture API calls, token counts, latency, and tool invocations. That's useful for debugging performance. It's useless for understanding decisions.

When you open a codebase that an agent worked on last Tuesday and something looks wrong, you want to know:

- Why did it choose this approach over the obvious one?
- What did it try that didn't work?
- What was it told to defer?

That information doesn't exist anywhere. Not in the traces. Not in the JSONL session files. Not in git history. It evaporates the moment the session ends.

This problem compounds in three directions:

**Across sessions** — agents start cold every time. They re-explore decisions already made, re-hit the same dead ends, re-justify the same architectural choices. There's no memory of _reasoning_, only of _output_.

**Across teammates** — when a human picks up work an agent started, they inherit the code but not the context. Why is auth structured this way? Why was the simpler approach abandoned? No trail.

**Across tools** — Claude Code, LangChain, custom agents, AutoGen — each has its own session format, its own log structure, its own conventions. Nothing is portable or queryable across them.

---

## The Solution

`agentlog` is a **local-first, framework-agnostic decision log daemon** for agentic workflows.

It runs as a lightweight background process on your machine. Any agent — Claude Code, a custom Python agent, a LangGraph workflow — can write structured decisions to it via a simple CLI or SDK. Those decisions are indexed, queryable, and persistent across sessions.

Think of it as the `.git` directory for agent reasoning. It lives on disk, belongs to the developer, requires no cloud account, and integrates with whatever you're already using.

---

## Core Concepts

### The Decision Log

The fundamental unit is a **decision entry** — not a trace span, not an API call log, but an explicit record of a choice made and why:

```json
{
  "id": "dec_a3f8b2",
  "session": "abc123",
  "timestamp": "2026-03-14T09:41:22Z",
  "type": "decision",
  "summary": "Switched from direct DB query to cached layer",
  "reason": "Hit rate limit on third consecutive attempt",
  "alternatives_considered": ["retry with backoff", "queue the request"],
  "outcome": "adopted",
  "files": ["src/db/cache.ts", "src/db/queries.ts"],
  "tags": ["architecture", "performance"]
}
```

Entry types include:

- `decision` — a fork in the road, with chosen path and reasoning
- `attempt_failed` — something tried and abandoned, with why
- `deferred` — a task explicitly left for later, with conditions
- `assumption` — a premise the agent is working from
- `question` — something unresolved the agent flagged

### The Daemon

A small, persistent background process (`agentlogd`) that:

- Owns the log and serialises concurrent writes from parallel agent runs
- Maintains indexes for fast querying
- Optionally watches `~/.claude/projects/` and auto-ingests Claude Code sessions
- Exposes a local Unix socket and HTTP endpoint for SDK integrations
- Has negligible resource footprint (target: <10MB RAM, <0.1% CPU)

### The Store

Logs are written to `~/.agentlog/` by default, or `.agentlog/` project-local if preferred. Format is append-only JSONL — human-readable, git-committable, trivially backed up.

```
~/.agentlog/
├── sessions/
│   ├── abc123.jsonl
│   └── def456.jsonl
├── index.db          # SQLite index for fast queries
└── config.toml
```

---

## How It's Used

### Writing decisions (during a run)

**From the CLI:**

```bash
agentlog write \
  --session abc123 \
  --type decision \
  --summary "Chose REST over GraphQL" \
  --reason "Client spec required REST, team unfamiliar with GraphQL" \
  --files "src/api/routes.ts" \
  --tags "architecture,api"
```

**From Python:**

```python
import agentlog

agentlog.write(
    type="attempt_failed",
    summary="Tried extracting auth middleware, broke 3 tests",
    reason="Session token logic was coupled to request object shape",
    next="Refactor session model first"
)
```

**From TypeScript:**

```typescript
import { agentlog } from 'agentlog-sdk'

await agentlog.write({
  type: 'deferred',
  summary: 'Rate limiting not implemented',
  reason: 'Out of scope for current sprint per spec',
  condition: 'Required before production deployment',
})
```

**From Claude Code (CLAUDE.md hook):**

```markdown
## Logging decisions

When you make a significant architectural decision, abandon an approach,
or defer a task, write it to agentlog:

agentlog write --type decision --summary "..." --reason "..."
```

### Querying (after a run)

```bash
# Chronological log of decisions
agentlog log

# Filter by time
agentlog log --since "7 days ago"

# Search across all sessions
agentlog query "auth middleware"

# Full detail on a session
agentlog show session abc123

# All decisions that touched a file
agentlog blame src/auth/session.ts

# Everything deferred across all sessions
agentlog log --type deferred

# Export to markdown (for docs, PRs, handoffs)
agentlog export --session abc123 --format markdown
```

`agentlog blame` is the flagship command: like `git blame` shows who changed a line, `agentlog blame` shows which agent run changed this file and what was reasoned at the time.

### Feeding context into the next run

```python
# At session start, pull relevant decision history
context = agentlog.context(
    files=["src/auth/"],
    limit=10,
    types=["decision", "deferred", "attempt_failed"]
)

prompt = f"""
Continue the auth refactor.

Prior decision history:
{context}

Current task: Extract the session middleware
"""
```

Output of `agentlog.context()`:

```
- [3 sessions ago] ATTEMPT_FAILED: Tried extracting middleware directly —
  session token logic was coupled to request object shape. Deferred.
- [2 sessions ago] DECISION: Refactored session model to decouple from
  request. Now ready for extraction.
- [last session] DEFERRED: Middleware extraction blocked on test coverage —
  condition: >80% coverage on auth module before proceeding.
```

---

## Use Cases

### 1. Solo developer using Claude Code

The most common case. You work across multiple sessions over several days. Without agentlog, every new session starts with "remind me where we got to." With agentlog, you run `agentlog log` and get the last 10 decisions in plain English. You paste `agentlog context --files src/` into your prompt and the agent picks up exactly where it left off, without re-exploring dead ends.

### 2. Handoff between agent and human

A Claude Code session completes a sprint's worth of work. The developer reviewing it runs `agentlog export --format markdown` and gets a readable narrative: what was built, what was tried and abandoned, what was deferred and why. This becomes the PR description, the architecture doc, or the sprint retrospective.

### 3. Multi-agent pipeline

A pipeline runs three agents in sequence: planner, implementer, reviewer. Each writes decisions to agentlog. The implementer reads the planner's decisions before starting. The reviewer reads both. Each agent has full context of what was decided upstream and why — without passing enormous context windows between them.

### 4. Team using agents on shared codebase

`.agentlog/` is committed to the repo. Any team member can run `agentlog log --since sprint-start` to see every architectural decision made by agents during the sprint. New team members can run `agentlog blame src/` to understand why the codebase is structured the way it is. The decision history is a first-class project artifact.

### 5. Debugging unexpected agent behaviour

An agent refactored something it shouldn't have. You run `agentlog show session abc123` and find the decision entry: "Refactored UserSession to extend BaseModel — reason: inconsistency with rest of domain layer." You now know it wasn't a bug, it was an autonomous architectural call. You can accept, revert, or add a constraint to CLAUDE.md to prevent it happening again.

### 6. Consulting / client delivery

Agents are increasingly doing real delivery work. Clients want an audit trail — not of API calls, but of decisions made on their behalf. `agentlog export --format pdf` becomes a deliverable: a structured record of every architectural choice the agentic workflow made, with reasoning, over the course of an engagement. This is something no existing tool produces.

---

## What It Is Not

**Not a tracing tool.** agentlog doesn't capture API calls, token counts, or latency. That's what Langfuse is for. They're complementary — use both if you need production observability.

**Not a replay system.** It doesn't let you re-run a session. It tells you what was decided, not what can be undone.

**Not Claude Code-specific.** It works with any agent that can make a CLI call or HTTP request. Claude Code is the primary target initially, but the design is explicitly framework-agnostic.

**Not a cloud product.** Local-first is a core constraint, not a roadmap item. Sync is opt-in. Your decision logs belong to you by default.

---

## Technical Design (v0 scope)

| Component       | Choice                   | Rationale                                          |
| --------------- | ------------------------ | -------------------------------------------------- |
| Daemon language | Go or Rust               | Low resource footprint, single binary distribution |
| Log format      | Append-only JSONL        | Human-readable, git-friendly, trivially parsed     |
| Index           | SQLite (embedded)        | No external dependency, fast full-text search      |
| IPC             | Unix socket + local HTTP | Works across languages, familiar to SDK authors    |
| SDK languages   | Python, TypeScript (v0)  | Covers Claude Code and most agent frameworks       |
| Config          | TOML                     | Readable, no surprises                             |
| Distribution    | Homebrew + npm + pip     | Cover the three main developer surfaces            |

---

## MVP Scope

**Phase 1 — Core daemon + CLI**

- `agentlogd` — background daemon, Unix socket, JSONL store, SQLite index
- `agentlog write` — append a decision entry
- `agentlog log` — chronological view with filtering
- `agentlog query` — full-text search across all entries
- `agentlog show` — full detail on a session
- `agentlog blame <file>` — decision history for a file

**Phase 2 — SDKs**

- Python SDK (`pip install agentlog-sdk`)
- TypeScript SDK (`npm install agentlog-sdk`)
- Claude Code slash command (`/agentlog`)

**Phase 3 — Context API**

- `agentlog context` — structured summary for prompt injection
- `agentlog export` — markdown / JSON / PDF output
- Claude Code auto-ingestion from `~/.claude/projects/`

**Phase 4 — Team features**

- `.agentlog/` project-local mode with git integration
- `agentlog diff` — compare decision logs across branches
- Optional encrypted cloud sync

---

## Positioning

| Tool                 | What it captures                  | When you use it                      |
| -------------------- | --------------------------------- | ------------------------------------ |
| Langfuse / LangSmith | API calls, spans, token usage     | Production monitoring, cost tracking |
| git                  | Code changes                      | Understanding what changed           |
| **agentlog**         | **Agent decisions and reasoning** | **Understanding why it changed**     |

The clearest summary: **the missing layer between git history and LLM traces**.

---

## Open Source Strategy

- MIT licence
- Single-binary install, zero cloud dependency — removes friction entirely
- CLAUDE.md snippet published as a standard pattern
- Integration guide for Claude Code, LangChain, LangGraph, AutoGen
- `agentlog.schema.json` published as a community spec — invite other tools to adopt the format

The long-term network effect: if enough agents write to the same schema, `agentlog blame` becomes a universal command that works regardless of which framework ran the agent.

---

## Why Now

- Claude Code has normalised agents doing real delivery work — but the tooling for understanding that work hasn't caught up
- The JSONL session format is already there; the daemon and query layer on top is the missing piece
- Framework fragmentation (Claude Code, LangGraph, AutoGen, custom) creates demand for something framework-agnostic
- Consulting and enterprise adoption of agents is accelerating demand for audit trails that clients can actually read

The observability market went from zero to crowded in 18 months. The _decision log_ layer hasn't started yet.
