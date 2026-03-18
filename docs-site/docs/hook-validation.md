# Hook Validation Guide

This guide covers how to verify that the agentlog Claude Code hooks are installed and working correctly.

## Expected behavior

The hooks are **silent by default**. They exit 0 and produce no output in all cases, including errors. This is intentional - hooks should never interfere with Claude Code sessions.

- **session-start.sh** runs on `UserPromptSubmit` and injects past decisions into the conversation context. It runs once per session.
- **decision-write.sh** runs on `Stop` events and captures file changes as decision entries. It runs after every Claude Code response.

## Verifying hook installation

### 1. Check that hooks are configured

Inspect your `.claude/settings.json` (project-level) or `~/.claude/settings.json` (global):

```bash
cat .claude/settings.json | jq '.hooks'
```

You should see entries for both `UserPromptSubmit` and `Stop` events pointing to the hook scripts.

### 2. Check that hook scripts are executable

```bash
ls -la .claude/hooks/session-start.sh .claude/hooks/decision-write.sh
```

Both scripts need the executable bit set (`chmod +x`).

### 3. Use verbose mode

Set `AGENTLOG_VERBOSE=1` in your environment before starting Claude Code. The hooks will print diagnostic output to stderr at each step:

```bash
AGENTLOG_VERBOSE=1 claude
```

With verbose mode enabled, you will see output like:

```
[agentlog] session_id=abc123
[agentlog] git repo detected at /path/to/repo
[agentlog] detected 3 file(s) from git state
[agentlog] found 2 new file(s) this turn
[agentlog] command: agentlog write --type decision --title "Modified foo.go bar.go" --files foo.go,bar.go --tags claude-code
[agentlog] decision written (session total: 1)
[agentlog] Session summary: 1 decisions captured
```

### 4. Use dry-run mode

Set `AGENTLOG_DRY_RUN=1` to see what the hooks would do without actually writing to the log:

```bash
AGENTLOG_DRY_RUN=1 AGENTLOG_VERBOSE=1 claude
```

## Verifying decisions are captured

After a Claude Code session where files were modified:

```bash
# Check recent log entries
agentlog log --limit 5

# Filter to decisions tagged by Claude Code
agentlog log --type decision --tags claude-code --limit 5
```

You should see entries with titles like "Modified foo.go bar.go" and the tag `claude-code`.

## Verifying context injection

Start a new Claude Code session with verbose mode in a repo that has existing agentlog decisions:

```bash
AGENTLOG_VERBOSE=1 claude
```

On the first prompt, you should see:

```
[agentlog] limit=10
[agentlog] git repo detected at /path/to/repo
[agentlog] detected 2 file(s) from git state
[agentlog] command: agentlog context --limit 10 --files foo.go --files bar.go
[agentlog] injecting # Relevant decisions...
[agentlog] Session summary: 3 context entries injected
```

## Session telemetry

Both hooks track per-session counters stored in `/tmp/agentlog-*`:

- **decision-write.sh** counts successful `agentlog write` calls in `/tmp/agentlog-decisions/{session_id}.count`
- **session-start.sh** stores the context entry count in `/tmp/agentlog-session-{session_id}`

These counters are reported via verbose mode with `Session summary:` lines.

## Troubleshooting

### Daemon not running

**Symptom**: Hooks run silently but no decisions are captured.

**Fix**: Start the daemon:

```bash
agentlog start
```

Verify it is running:

```bash
agentlog log
# Should not error (empty log is fine)
```

### agentlog not on PATH

**Symptom**: Verbose mode shows `agentlog not found on PATH, exiting`.

**Fix**: Ensure the agentlog binary is on your PATH. If installed via the install script, the binary location depends on your installation method:

```bash
which agentlog
```

### Not in a git repository

**Symptom**: decision-write.sh shows `not in a git repo, exiting` in verbose mode.

**Fix**: The decision-write hook requires a git repository to detect file changes. Run Claude Code from within a git repository.

The session-start hook works outside git repos by falling back to topic-based queries.

### No modified files

**Symptom**: Verbose mode shows `detected 0 file(s) from git state` or `no new changes, exiting`.

**Cause**: This is normal when Claude Code responds without modifying files (e.g., answering a question). The decision-write hook only captures file changes.

### Hook not firing

**Symptom**: No verbose output at all, even with `AGENTLOG_VERBOSE=1`.

**Fix**: Check that:

1. The hook is configured in `.claude/settings.json` (see installation steps above)
2. The hook script path in settings.json matches the actual file location
3. The script has execute permissions
4. Claude Code is picking up the correct settings file (project vs. global)

### Session marker preventing re-injection

**Symptom**: session-start.sh only injects context on the first prompt of a session.

**Cause**: This is by design. The hook uses a marker file at `/tmp/agentlog-session-{session_id}` to run only once per session. To force re-injection during testing, remove the marker file and restart the session.
