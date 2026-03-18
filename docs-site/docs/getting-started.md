# Getting Started

## Installation

### Homebrew (macOS/Linux)

```bash
brew install byronxlg/agentlog/agentlog
```

### Python SDK

```bash
pip install agentlog-sdk
```

### TypeScript SDK

```bash
npm install agentlog-sdk
```

### Build from source

Requires Go 1.23+:

```bash
git clone https://github.com/byronxlg/agentlog.git
cd agentlog
make build
```

This produces two binaries in `bin/`: `agentlog` (CLI) and `agentlogd` (daemon).

Add them to your PATH:

```bash
export PATH="$PWD/bin:$PATH"
```

## Start the daemon

agentlog uses a background daemon to manage writes and queries. Start it:

```bash
agentlog start
```

You should see output like:

```
agentlogd started (PID 12345)
```

## Write a decision

Log your first decision entry:

```bash
agentlog write --type decision \
  --title "Use SQLite for the index" \
  --body "Considered PostgreSQL but SQLite keeps us single-binary and local-first." \
  --tags "architecture,database" \
  --files "internal/index/index.go"
```

The command prints the entry ID on success.

## Query it back

List recent entries:

```bash
agentlog log
```

Full-text search:

```bash
agentlog query "SQLite"
```

See decisions related to a specific file:

```bash
agentlog blame internal/index/index.go
```

## Export decisions

Generate formatted output for PR descriptions, retrospectives, or handoff documents:

```bash
# PR summary from today's work
agentlog export --template pr --since 1d

# Full markdown export of the last week
agentlog export --since 7d

# Export as JSON for scripting
agentlog export --format json --since 7d
```

## Stop the daemon

When you are done:

```bash
agentlog stop
```

## Next steps

- See the full [CLI Reference](cli-reference.md) for all commands and flags
- Set up [Claude Code Integration](claude-code.md) for automatic decision logging
- Use the [Python SDK](python-sdk.md) or [TypeScript SDK](typescript-sdk.md) for programmatic access
