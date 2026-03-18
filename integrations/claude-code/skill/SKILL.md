---
name: agentlog
description: Decision log for agentic workflows. Use agentlog to capture decisions, query past context, and understand why changes were made. Self-configures Claude Code hooks for automatic decision capture and context injection.
---

# agentlog

A local-first decision log for agentic workflows. Captures decisions and reasoning
as you work - the missing layer between git history and LLM traces.

Use agentlog to record why changes were made, not just what changed.

## When to use

- **Before making changes:** Query past decisions for context on the area you are
  working in. This prevents re-litigating settled decisions or repeating failed
  approaches.
- **After making changes:** Log the decision with rationale so future sessions
  (yours or another agent's) understand why.
- **When deferring work:** Record what was deferred and why, so it is not lost.
- **When something fails:** Log failed attempts so they are not repeated.

## Prerequisites

The agentlog daemon must be running. If a command fails with a connection error,
start the daemon:

```bash
agentlog start
```

If `agentlog` is not on PATH, all commands below will fail silently or with
"command not found". Install with `brew install byronxlg/agentlog/agentlog` or
build from source.

## CLI commands

### write - Record a decision

Use `write` after making a change or reaching a decision worth preserving.

```bash
agentlog write --type decision \
  --title "Use connection pooling for database access" \
  --body "Single connections caused timeouts under load. Pool size set to 10 based on benchmark results." \
  --files "internal/db/pool.go,internal/db/config.go" \
  --tags "database,performance"
```

Flags:

| Flag | Required | Description |
|------|----------|-------------|
| `--type` | Yes | Entry type (see entry types below) |
| `--title` | Yes | Short description of the decision |
| `--body` | No | Detailed rationale |
| `--files` | No | Comma-separated file paths involved |
| `--tags` | No | Comma-separated tags for categorization |
| `--session` | No | Session ID to append to (creates new session if omitted) |

### log - List entries with filters

Use `log` to browse recent decisions or filter by specific criteria.

```bash
# Recent decisions tagged "api"
agentlog log --tag api --limit 10

# Decisions from the last 24 hours
agentlog log --since 24h

# Decisions affecting a specific file
agentlog log --file internal/api/handler.go

# Verbose output showing entry bodies
agentlog log --tag api --verbose
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | | Filter by entry type |
| `--session` | | Filter by session ID |
| `--tag` | | Filter by tag |
| `--file` | | Filter by referenced file path |
| `--since` | | Entries after this time (RFC3339 or relative: `1h`, `7d`) |
| `--until` | | Entries before this time (RFC3339 or relative: `1h`, `7d`) |
| `--verbose` | `false` | Show entry body text inline |
| `--limit` | `50` | Maximum entries to display |
| `--offset` | `0` | Number of entries to skip |

### query - Full-text search

Use `query` to search across all decision content when you do not know the exact
tag or file.

```bash
# Search for decisions about authentication
agentlog query "authentication"

# Narrow search to a specific type
agentlog query "timeout" --type decision --limit 5
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | | Filter by entry type |
| `--session` | | Filter by session ID |
| `--tag` | | Filter by tag |
| `--file` | | Filter by file reference |
| `--since` | | Entries after this time |
| `--until` | | Entries before this time |
| `--limit` | `20` | Maximum results |

Positional argument: search term (required).

### show - View a session

Use `show` to see all entries in a session. Useful to understand the full arc of
a previous work session.

```bash
agentlog show abc123
```

Accepts a session ID or a unique prefix of one. No flags.

### blame - Decisions affecting a file

Use `blame` before modifying a file to understand past decisions about it.

```bash
# See decisions that reference this file
agentlog blame internal/api/handler.go

# With full body text
agentlog blame --verbose internal/api/handler.go
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--verbose` | `false` | Show entry body text inline |

Positional argument: file path (required, resolved to absolute path).

### context - Get decisions for LLM context

Use `context` to retrieve formatted decisions relevant to specific files or a
topic. This is what the session-start hook calls automatically.

```bash
# Decisions relevant to specific files
agentlog context --files src/App.tsx --files src/api.ts --limit 5

# Decisions relevant to a topic
agentlog context --topic "authentication" --limit 5
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--files` | | File path to look up (repeatable) |
| `--topic` | | Search string for full-text search |
| `--limit` | `10` | Maximum entries to return |
| `--json` | `false` | Output raw JSON instead of formatted text |

At least one of `--files` or `--topic` is required.

## Entry types

Use the type that best describes what happened:

### decision

A deliberate choice between alternatives. The most common type.

**When to use:** After choosing an approach, technology, pattern, or design.

```bash
agentlog write --type decision \
  --title "Use SQLite over PostgreSQL for local index" \
  --body "Local-first design requires zero-config storage. SQLite via modernc.org/sqlite avoids CGO dependency." \
  --tags "architecture,storage" \
  --files "internal/store/sqlite.go"
```

### attempt_failed

Something was tried and did not work. Prevents future agents from repeating the
same mistake.

**When to use:** After an approach fails during implementation or testing.

```bash
agentlog write --type attempt_failed \
  --title "Tried using fsnotify for file watching" \
  --body "fsnotify missed rename events on macOS. Switched to polling with 500ms interval." \
  --tags "file-watching" \
  --files "internal/watcher/watcher.go"
```

### deferred

Work that was identified but intentionally postponed. Creates a breadcrumb for
future sessions.

**When to use:** When you notice something that needs doing but is out of scope
for the current task.

```bash
agentlog write --type deferred \
  --title "Config file migration from TOML to YAML" \
  --body "Current TOML config works fine. Migration adds risk with no immediate benefit. Revisit if users request YAML." \
  --tags "config,tech-debt"
```

### assumption

A belief taken as true without full verification. Flagging assumptions makes them
auditable and easy to revisit.

**When to use:** When proceeding based on something you have not fully confirmed.

```bash
agentlog write --type assumption \
  --title "Daemon socket path is writable by current user" \
  --body "Using /tmp for socket file. Assuming standard unix permissions. May need XDG_RUNTIME_DIR on some Linux distros." \
  --tags "daemon,unix" \
  --files "internal/daemon/server.go"
```

### question

An open question that needs human input or further investigation.

**When to use:** When blocked or uncertain and the answer affects your approach.

```bash
agentlog write --type question \
  --title "Should the CLI support Windows named pipes?" \
  --body "Unix domain sockets do not exist on Windows. Need to decide: named pipes, TCP localhost, or skip Windows support." \
  --tags "platform,cli"
```

## Common workflows

### Starting a session

Before diving into code, check what decisions exist for the area you are working in:

```bash
# Check decisions for files you will modify
agentlog blame src/api/handler.go

# Or search by topic
agentlog query "API rate limiting"
```

### During implementation

Log decisions as you make them. Do not batch them up at the end.

```bash
agentlog write --type decision \
  --title "Rate limit uses token bucket algorithm" \
  --body "Sliding window was too memory-intensive for per-user tracking. Token bucket gives comparable fairness with O(1) memory per user." \
  --files "src/api/ratelimit.go" \
  --tags "api,rate-limiting"
```

### When something fails

```bash
agentlog write --type attempt_failed \
  --title "Redis-based rate limiter had too much latency" \
  --body "P99 latency jumped from 5ms to 45ms. In-process token bucket is fast enough for single-instance deployment." \
  --files "src/api/ratelimit.go" \
  --tags "api,rate-limiting,performance"
```

### Reviewing recent activity

```bash
# What happened in the last day?
agentlog log --since 24h

# What decisions were tagged with the current feature?
agentlog log --tag api --verbose
```

## Graceful behavior when daemon is not running

All agentlog commands exit cleanly when the daemon is not running. The hooks
(`session-start.sh` and `decision-write.sh`) also exit silently with code 0
when:

- `agentlog` is not on PATH
- The daemon is not running
- No git repo is detected (decision-write only)
- No relevant decisions are found (session-start only)

This means agentlog never blocks or breaks your workflow. If commands return
errors about connecting to the daemon, run `agentlog start`.

## Hook self-configuration

agentlog integrates with Claude Code through two hooks that automate context
injection and decision capture. Check if they are already configured before
setting them up.

### Check existing configuration

Look for agentlog hooks in your Claude Code settings:

```bash
grep -q "session-start\|decision-write" .claude/settings.json 2>/dev/null && echo "already configured" || echo "not configured"
```

If hooks are already present, no action is needed.

### Automated installation

The simplest approach is the install script:

```bash
# Project-level (recommended)
bash integrations/claude-code/install.sh

# Global (all projects)
bash integrations/claude-code/install.sh --global
```

This copies hook scripts to `.claude/hooks/` and patches `settings.json`.
Requires `jq`.

### Manual configuration via update-config skill

If the install script is not available or you need finer control, use the
`update-config` skill to add hooks to `settings.json`:

1. **Session-start hook** (context injection) - runs on `UserPromptSubmit`:

   Tell the `update-config` skill to add a `UserPromptSubmit` hook with:
   - matcher: `""` (all prompts)
   - type: `command`
   - command: `integrations/claude-code/session-start.sh`

2. **Decision-write hook** (automatic capture) - runs on `Stop`:

   Tell the `update-config` skill to add a `Stop` hook with:
   - matcher: `""` (all responses)
   - type: `command`
   - command: `integrations/claude-code/decision-write.sh`

For global installation, specify that the hooks should be added to the global
settings file (`~/.claude/settings.json`) instead of the project-level one.

### What the hooks do

- **session-start.sh** runs on the first prompt of each session. It detects your
  working set of files from git state and queries the daemon for relevant past
  decisions, injecting them as conversation context.

- **decision-write.sh** runs after each Claude Code response. It compares git
  state against a per-session snapshot to detect newly modified files and logs
  them as decisions automatically.

Both hooks are silent by default and never interfere with normal Claude Code
operation. See `integrations/claude-code/README.md` for environment variables
and customization options.
