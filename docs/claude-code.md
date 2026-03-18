# Using agentlog with Claude Code

agentlog captures the decisions and reasoning that happen during agentic coding sessions. This guide shows how to integrate it with Claude Code so that decisions are logged automatically as Claude works.

## Prerequisites

Install agentlog and start the daemon:

```bash
# Homebrew (recommended)
brew install byronxlg/agentlog/agentlog

# Or build from source
git clone https://github.com/byronxlg/agentlog.git
cd agentlog && make build
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

There are three ways to integrate agentlog with Claude Code, from simplest to most hands-on:

1. **Skill installation (recommended)** - Install the agentlog skill for Claude Code. Provides CLI guidance, usage examples, and self-configures hooks for automatic capture.
2. **Hook scripts only** - Install just the hooks for automatic decision capture and context injection without the skill's CLI guidance.
3. **CLAUDE.md instructions** - Tell Claude to log decisions via the CLI manually. No hooks, relies on Claude following instructions.

All three approaches can be combined.

## Approach 1: Skill installation (recommended)

The agentlog skill gives Claude Code full knowledge of the agentlog CLI and configures hooks automatically. This is the fastest way to get started.

### Install the skill

From the agentlog repo, install the skill into your project:

```bash
claude install-skill /path/to/agentlog/integrations/claude-code/skill
```

This installs a skill that teaches Claude Code:
- All agentlog CLI commands (`write`, `log`, `query`, `show`, `blame`, `context`)
- The five entry types and when to use each one
- Common workflows (starting sessions, capturing decisions, reviewing history)
- How to self-configure hooks for automatic capture

### Configure hooks

After installing the skill, ask Claude to set up the hooks:

```
Set up agentlog hooks for this project
```

Claude will check if hooks are already configured, and if not, run the install script or configure them manually. The hooks provide:

- **Context injection** - On the first prompt of each session, queries the daemon for decisions relevant to your current files and injects them as context
- **Decision capture** - After each Claude Code response, detects newly modified files and logs them as decisions automatically

### Verify the setup

```bash
# Check hooks are configured
grep -q "session-start\|decision-write" .claude/settings.json && echo "hooks configured"

# Check the skill is installed
ls .claude/skills/agentlog/
```

See [integrations/claude-code/README.md](../integrations/claude-code/README.md) for environment variables, customization, and troubleshooting.

## Approach 2: Hook scripts only

If you want automatic capture and context injection without installing the skill, you can install just the hooks. This is useful when you prefer explicit control over what Claude knows about agentlog.

### Automated setup

Run the install script from your project root:

```bash
bash /path/to/agentlog/integrations/claude-code/install.sh
```

This copies both hook scripts to `.claude/hooks/` and configures `.claude/settings.json`. For global installation (all projects), add `--global`.

See [integrations/claude-code/README.md](../integrations/claude-code/README.md) for manual configuration, environment variables, and customization.

### What the hooks provide

- **session-start.sh** (context injection) - Runs on `UserPromptSubmit`. On the first prompt of each session, detects your working files from git state and queries the daemon for relevant past decisions.
- **decision-write.sh** (automatic capture) - Runs on `Stop`. After each Claude Code response, compares git state against a per-session snapshot to detect newly modified files and logs them as decisions.

Both hooks are silent by default and never interfere with normal Claude Code operation.

## Approach 3: CLAUDE.md instructions

Add the following snippet to your project's `CLAUDE.md` file. Claude Code reads this file at the start of every conversation and follows its instructions. This approach relies on Claude remembering to log decisions rather than capturing them automatically.

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

The real power of agentlog is giving future sessions context about past decisions. The `agentlog context` command powers the session-start hook and returns decisions formatted as markdown:

```bash
agentlog context --files src/index.go --files internal/store.go --limit 10
agentlog context --topic my-project
```

SDKs also expose a `context()` method for programmatic access. See [sdk/python/README.md](../sdk/python/README.md) and [sdk/typescript/README.md](../sdk/typescript/README.md) for details.

Add this to your CLAUDE.md to encourage manual context checks:

```markdown
## Before making changes

Before modifying a file, check if there are logged decisions about it:

    agentlog blame <file>

This shows past decisions, failed attempts, and assumptions related to the file.
Use this context to avoid repeating failed approaches or contradicting prior decisions.
```

## Troubleshooting

**"daemon is not running"** - Start the daemon with `agentlog start`. Check `~/.agentlog/agentlogd.log` for errors.

**Decisions not appearing** - Verify the daemon is running (`agentlog log` should not error). Check that the `--type` flag uses a valid value: decision, attempt_failed, deferred, assumption, or question.

**Hook not firing** - Verify the hook script is executable (`chmod +x`). Check that the matcher pattern in settings.json matches the tools you want to capture.

**No context appearing** - Run `AGENTLOG_VERBOSE=1 bash .claude/hooks/session-start.sh` to see verbose output. Check the troubleshooting section in [integrations/claude-code/README.md](../integrations/claude-code/README.md).
