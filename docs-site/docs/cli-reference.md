# CLI Reference

All commands accept the `--dir <path>` global flag to override the data directory (default: `~/.agentlog`).

```
agentlog [--dir <path>] <command>
```

## Commands

### `agentlog start`

Start the agentlogd daemon as a background process. The daemon listens on a Unix socket and manages all reads and writes.

```bash
agentlog start
```

If the daemon is already running, the command exits with an error showing the existing PID. Stale PID files from crashed daemons are cleaned up automatically.

### `agentlog stop`

Stop the running agentlogd daemon by sending SIGTERM and waiting for it to exit (up to 5 seconds).

```bash
agentlog stop
```

### `agentlog write`

Write a decision entry to the log. Requires `--type` and `--title`.

```bash
agentlog write --type <type> --title <title> [flags]
```

**Flags:**

| Flag | Required | Description |
|------|----------|-------------|
| `--type` | Yes | Entry type (see [Entry types](#entry-types) below) |
| `--title` | Yes | Short description of the decision |
| `--body` | No | Longer explanation with reasoning and context |
| `--tags` | No | Comma-separated tags (e.g. `"performance,sqlite"`) |
| `--files` | No | Comma-separated file paths affected by this decision |
| `--session` | No | Session ID to append to. If omitted, a new session is created |

**Example:**

```bash
agentlog write --type decision \
  --title "Use batch inserts for index rebuild" \
  --body "Batch is 10x faster for rebuild of 10k+ entries. Trade-off: more complex transaction handling." \
  --tags "performance,sqlite" \
  --files "internal/index/rebuild.go"
```

On success, prints the entry ID.

### `agentlog log`

List entries with optional filters. Results are sorted newest-first.

```bash
agentlog log [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | | Filter by entry type |
| `--session` | | Filter by session ID |
| `--tag` | | Filter by tag |
| `--since` | | Show entries after this time (RFC 3339 or relative: `1h`, `7d`, `30m`) |
| `--until` | | Show entries before this time (RFC 3339 or relative: `1h`, `7d`, `30m`) |
| `--file` | | Filter by referenced file path |
| `--verbose` | `false` | Show entry body text inline |
| `--limit` | `50` | Maximum number of entries to display |
| `--offset` | `0` | Number of entries to skip (for pagination) |

**Examples:**

```bash
# Recent decisions
agentlog log --type decision

# Entries from the last hour
agentlog log --since 1h

# Entries tagged "infrastructure" with body text
agentlog log --tag infrastructure --verbose

# Paginate: skip first 50, show next 50
agentlog log --offset 50 --limit 50
```

### `agentlog query`

Full-text search across entry titles and bodies. Accepts the same filter flags as `log`. Results are displayed with matching text highlighted.

```bash
agentlog query [flags] <search_term>
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | | Filter by entry type |
| `--session` | | Filter by session ID |
| `--tag` | | Filter by tag |
| `--since` | | Show entries after this time (RFC 3339 or relative: `1h`, `7d`, `30m`) |
| `--until` | | Show entries before this time (RFC 3339 or relative: `1h`, `7d`, `30m`) |
| `--file` | | Filter by file reference |
| `--limit` | `20` | Maximum number of results |
| `--socket` | | Override daemon socket path |

**Examples:**

```bash
agentlog query "sqlite performance"
agentlog query "database" --type decision --limit 5
agentlog query "migration" --since 7d
```

### `agentlog show`

Show all entries in a session, sorted chronologically. Supports prefix matching on session IDs.

```bash
agentlog show <session_id>
```

**Example:**

```bash
# Full session ID
agentlog show a1b2c3d4-e5f6-7890-abcd-ef1234567890

# Prefix match
agentlog show a1b2c3
```

If the prefix matches multiple sessions, the command lists the ambiguous matches and exits with an error.

### `agentlog blame`

Show all decisions referencing a given file path. Results are sorted newest-first.

```bash
agentlog blame [--verbose] <file>
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--verbose` | Show entry body text inline |

**Example:**

```bash
agentlog blame internal/index/index.go
agentlog blame --verbose src/main.go
```

### `agentlog context`

Get relevant decisions formatted for LLM context injection. Returns decisions as a markdown-formatted text block, filtered by files or topic.

```bash
agentlog context [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--files` | | Comma-separated file paths to find relevant decisions for |
| `--topic` | | Topic or project name to search for |
| `--limit` | `10` | Maximum number of entries to return |

**Examples:**

```bash
# Context for specific files
agentlog context --files src/index.go,internal/store.go

# Context for a topic
agentlog context --topic authentication

# Combine both
agentlog context --files src/auth.go --topic "session tokens" --limit 5
```

### `agentlog export`

Export decision entries as formatted output. Supports multiple output formats and built-in templates for common use cases.

```bash
agentlog export [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--session` | | Filter by session ID |
| `--since` | | Show entries after this time (RFC 3339 or relative: `1h`, `7d`) |
| `--until` | | Show entries before this time (RFC 3339 or relative: `1h`, `7d`) |
| `--file` | | Filter by referenced file path |
| `--tag` | | Filter by tag |
| `--type` | | Filter by entry type |
| `--format` | `markdown` | Output format: `markdown`, `json`, `text` |
| `--template` | | Use a built-in template: `pr`, `retro`, `handoff` |

**Templates:**

| Template | Description |
|----------|-------------|
| `pr` | PR summary - decisions and changes for a pull request description |
| `retro` | Retrospective - categorized review of decisions, failed attempts, and deferred work |
| `handoff` | Handoff document - context needed for another developer to continue the work |

**Examples:**

```bash
# Export recent decisions as markdown
agentlog export --since 1d

# Export as JSON for processing
agentlog export --format json --since 7d

# Generate a PR summary from today's session
agentlog export --template pr --since 1d

# Retrospective for the last week
agentlog export --template retro --since 7d

# Handoff document filtered by file
agentlog export --template handoff --file internal/auth/jwt.go

# Plain text export for a specific session
agentlog export --format text --session a1b2c3d4
```

## Entry types

Each entry has a type that describes the kind of decision being logged:

| Type | Use when |
|------|----------|
| `decision` | Choosing between alternatives |
| `attempt_failed` | Something you tried that did not work |
| `deferred` | Work you chose to skip or postpone |
| `assumption` | An assumption that could be wrong |
| `question` | An open question you cannot answer from context |

## Time formats

The `--since` and `--until` flags accept two formats:

- **Relative durations**: a number followed by a unit - `m` (minutes), `h` (hours), `d` (days). Examples: `30m`, `1h`, `7d`.
- **Absolute timestamps**: RFC 3339 format. Example: `2026-03-15T10:30:00Z`.

Relative durations are subtracted from the current time. For example, `--since 1h` means "entries from the last hour".
