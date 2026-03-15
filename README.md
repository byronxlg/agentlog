# agentlog

A local-first, framework-agnostic decision log daemon for agentic workflows. Captures agent decisions and reasoning - the missing layer between git history and LLM traces.

## Why

Git tells you *what* changed. LLM traces tell you *what was said*. Neither tells you *why* a decision was made, what alternatives were considered, or what failed before the final approach. agentlog fills that gap.

- **Append-only JSONL logs** - human-readable, git-committable, greppable
- **SQLite index** - fast queries by time, type, session, tags, or file
- **Full-text search** - find decisions by keyword across title and body
- **File blame** - see which decisions affected a specific file
- **Session tracking** - group related decisions within a coding session
- **Single binary** - pure Go, no CGO, no external dependencies

## Installation

Build from source (requires Go 1.23+):

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

## Quick start

Start the daemon:

```bash
agentlog start
```

Log a decision:

```bash
agentlog write --type decision \
  --title "Use SQLite for the index" \
  --body "Considered PostgreSQL but SQLite keeps us single-binary and local-first." \
  --tags "architecture,database" \
  --files "internal/index/index.go"
```

Query it back:

```bash
agentlog log
agentlog query "SQLite"
agentlog blame internal/index/index.go
```

Stop the daemon when done:

```bash
agentlog stop
```

## CLI reference

| Command | Description |
|---------|-------------|
| `agentlog start` | Start the agentlogd daemon |
| `agentlog stop` | Stop the agentlogd daemon |
| `agentlog write` | Write a decision entry (requires `--type` and `--title`) |
| `agentlog log` | List entries with optional filters (`--type`, `--session`, `--tag`, `--since`, `--until`, `--file`) |
| `agentlog query <text>` | Full-text search across decision entries |
| `agentlog show <session>` | Show all entries in a session (supports prefix matching) |
| `agentlog blame <file>` | Show decisions referencing a file |

All commands accept `--dir <path>` to override the data directory (default: `~/.agentlog`).

### Entry types

| Type | Use when |
|------|----------|
| `decision` | Choosing between alternatives |
| `attempt_failed` | Something you tried that didn't work |
| `deferred` | Work you chose to skip or postpone |
| `assumption` | An assumption that could be wrong |
| `question` | An open question you can't answer from context |

## Python SDK

Install the Python SDK for programmatic access:

```bash
pip install agentlog
```

```python
from agentlog import AgentLog

log = AgentLog()
session = log.create_session()

session.decision(
    title="Use batch inserts for rebuild",
    body="10x faster for 10k+ entries",
    tags=["performance"],
    files=["internal/index/rebuild.go"],
)

entries = log.query(type="decision", since="1h")
```

See [sdk/python/README.md](sdk/python/README.md) for the full SDK documentation.

## Claude Code integration

agentlog integrates with Claude Code to automatically log decisions during coding sessions. Two approaches are supported:

1. **CLAUDE.md instructions** - Add a snippet to your CLAUDE.md that tells Claude to log decisions via the CLI
2. **Hook scripts** - Auto-log file modifications using Claude Code's hook system

See [docs/claude-code.md](docs/claude-code.md) for the full integration guide and examples.

## TypeScript SDK

Coming soon.

## Architecture

```
agentlog (CLI) --> agentlogd (daemon) --> JSONL files (source of truth)
                                      --> SQLite index (derived, rebuildable)
```

The daemon listens on a Unix socket, serializes writes to JSONL log files, and keeps a SQLite index updated for fast queries. The JSONL files are the source of truth - the index can be rebuilt from them at any time.

## License

TBD
