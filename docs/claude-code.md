# Using agentlog with Claude Code

agentlog captures the decisions and reasoning that happen during agentic coding sessions. This guide shows how to integrate it with Claude Code so that decisions are logged automatically as Claude works.

## Prerequisites

Install agentlog and make sure the binaries are on your PATH:

```bash
# Build from source
git clone https://github.com/byronxlg/agentlog.git
cd agentlog
make build

# Add to PATH
export PATH="$PWD/bin:$PATH"
```

Start the daemon:

```bash
agentlog start
```

Verify it's running:

```bash
agentlog log
# Should print "no entries found" (empty log is fine)
```

## Integration approaches

There are two ways to integrate agentlog with Claude Code:

1. **CLAUDE.md instructions** - Tell Claude to log decisions via the CLI. Simple, no setup beyond editing a file.
2. **Hook scripts** - Automatically log decisions using Claude Code's hook system. More powerful, captures decisions without relying on Claude remembering to log them.

Both approaches can be used together.

## Approach 1: CLAUDE.md instructions

Add the following snippet to your project's `CLAUDE.md` file. Claude Code reads this file at the start of every conversation and follows its instructions.

```markdown
## Decision logging

This project uses agentlog to capture decision history. When working on code, log
significant decisions using the agentlog CLI:

- **decision** - A choice between alternatives (e.g., "Use SQLite over PostgreSQL for the index")
- **attempt_failed** - Something you tried that didn't work and why
- **deferred** - Something you chose not to do now, with reasoning
- **assumption** - An assumption you're making that could be wrong
- **question** - An open question that needs answering

Log decisions with:

    agentlog write --type <type> --title "<short description>" \
      --body "<reasoning and context>" \
      --tags "<comma-separated tags>" \
      --files "<comma-separated file paths affected>"

### When to log

- Before making a non-obvious choice between alternatives
- When you try something and it fails (log before moving on)
- When you defer work or skip a refactor
- When you assume something about the codebase or requirements
- When you have a question you can't answer from context

### Example

    agentlog write --type decision \
      --title "Use batch inserts for index rebuild" \
      --body "Considered per-entry inserts vs batch. Batch is 10x faster for rebuild of 10k+ entries. Trade-off: more complex transaction handling." \
      --tags "performance,sqlite" \
      --files "internal/index/rebuild.go"
```

### Full CLAUDE.md example

See `examples/claude-code-snippet.md` for a complete CLAUDE.md section you can copy into your project.

## Approach 2: Hook scripts

Claude Code supports hooks that run shell commands in response to events. You can use a hook to automatically log tool usage and decisions.

### Setup

1. Copy the hook script to your project:

```bash
cp examples/hooks/agentlog-hook.sh .claude/hooks/
chmod +x .claude/hooks/agentlog-hook.sh
```

2. Configure the hook in your Claude Code settings. Add to `.claude/settings.json`:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "hooks": [
          {
            "type": "command",
            "command": ".claude/hooks/agentlog-hook.sh"
          }
        ]
      }
    ]
  }
}
```

This logs a decision entry every time Claude edits or writes a file, capturing which files were modified and the tool that was used.

### Customizing the hook

The hook script receives tool use context via environment variables. Edit the script to change what gets logged, which tools trigger logging, or what tags are applied.

## Querying decisions

Once decisions are being logged, you can query them:

```bash
# View recent decisions
agentlog log

# Filter by type
agentlog log --type decision
agentlog log --type attempt_failed

# Filter by time
agentlog log --since 1h
agentlog log --since 7d

# Full-text search
agentlog query "sqlite performance"

# See all decisions in a session
agentlog show <session-id>

# See decisions related to a file
agentlog blame src/index.go
```

### Using decisions for context

The real power of agentlog is giving future sessions context about past decisions. Add this to your CLAUDE.md:

```markdown
## Before making changes

Before modifying a file, check if there are logged decisions about it:

    agentlog blame <file>

This shows past decisions, failed attempts, and assumptions related to the file.
Use this context to avoid repeating failed approaches or contradicting prior decisions.
```

## Python SDK (coming soon)

A Python SDK is in development that provides a programmatic interface to agentlog. This will enable richer integrations, including:

- Structured decision logging from Python scripts
- Querying decisions programmatically
- Building custom tools that read and write decision logs

Check the project repository for updates on SDK availability.

## Troubleshooting

**"daemon is not running"** - Start the daemon with `agentlog start`. Check `~/.agentlog/agentlogd.log` for errors.

**Decisions not appearing** - Verify the daemon is running (`agentlog log` should not error). Check that the `--type` flag uses a valid value: decision, attempt_failed, deferred, assumption, or question.

**Hook not firing** - Verify the hook script is executable (`chmod +x`). Check that the matcher pattern in settings.json matches the tools you want to capture.
