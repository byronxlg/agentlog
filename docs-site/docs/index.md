# agentlog

A local-first, framework-agnostic decision log daemon for agentic workflows. Captures agent decisions and reasoning - the missing layer between git history and LLM traces.

Git tells you *what* changed. LLM traces tell you *what was said*. Neither tells you *why* a decision was made, what alternatives were considered, or what failed before the final approach. agentlog fills that gap.

## Key features

- **Append-only JSONL logs** - human-readable, git-committable, greppable
- **SQLite index** - fast queries by time, type, session, tags, or file
- **Full-text search** - find decisions by keyword across title and body
- **File blame** - see which decisions affected a specific file
- **Session tracking** - group related decisions within a coding session
- **Single binary** - pure Go, no CGO, no external dependencies

## Quick links

- [Getting Started](getting-started.md) - install agentlog and log your first decision
- [CLI Reference](cli-reference.md) - all commands, flags, and entry types
- [Python SDK](python-sdk.md) - programmatic access from Python
- [TypeScript SDK](typescript-sdk.md) - programmatic access from TypeScript
- [Claude Code Integration](claude-code.md) - automatic decision logging with Claude Code
- [Architecture](architecture.md) - how agentlog works internally
