# Validating your agentlog hooks

This guide helps you verify that the Claude Code hooks are installed and working correctly.

## Quick check

Run these commands from your project root to verify the basics:

```bash
# 1. Verify agentlog is installed and on PATH
which agentlog

# 2. Verify the daemon is running
agentlog log
# Expected: a list of entries, or "no entries found" (not an error)

# 3. Verify hook scripts exist and are executable
ls -la .claude/hooks/session-start.sh .claude/hooks/decision-write.sh

# 4. Verify settings.json references the hooks
cat .claude/settings.json | jq '.hooks'
```

## Verbose mode

Both hooks support `AGENTLOG_VERBOSE=1` for diagnostic output. Set it as an environment variable before starting Claude Code:

```bash
export AGENTLOG_VERBOSE=1
claude
```

With verbose mode enabled, the hooks print diagnostic messages to stderr at each step.

### session-start.sh verbose output

```
[agentlog] limit=10
[agentlog] git repo detected at /path/to/repo
[agentlog] detected 3 file(s) from git state
[agentlog] after dedup: 2 unique file(s)
[agentlog] command: agentlog context --limit 10 --files file1.go --files file2.go
[agentlog] injecting # Relevant decisions...
[agentlog] Session summary: 3 context entries injected
```

### decision-write.sh verbose output

```
[agentlog] session_id=abc123
[agentlog] git repo detected at /path/to/repo
[agentlog] detected 2 file(s) from git state
[agentlog] after dedup: 2 unique file(s)
[agentlog] found 1 new file(s) this turn
[agentlog] command: agentlog write --type decision --title "Modified file.go" --files file.go --tags claude-code
[agentlog] decision written
[agentlog] Session summary: 3 decisions captured
```

## Verifying context injection

1. Seed a test decision:

```bash
agentlog write --type decision \
  --title "Test decision for validation" \
  --body "This is a test to verify context injection works." \
  --tags "test" \
  --files "README.md"
```

2. Modify README.md (or ensure it appears in your git working set):

```bash
echo "" >> README.md
```

3. Start Claude Code and send a prompt. With `AGENTLOG_VERBOSE=1`, you should see the context injection messages. The test decision should appear in the conversation context.

4. Clean up:

```bash
git checkout README.md
```

## Verifying decision capture

1. Start a Claude Code session and ask Claude to make a small change to any file.

2. After Claude responds, check for captured decisions:

```bash
agentlog log --limit 5
```

You should see an entry tagged `claude-code` with the files Claude modified.

3. With verbose mode, the decision-write hook confirms each capture:

```
[agentlog] decision written
[agentlog] Session summary: 1 decisions captured
```

## Telemetry

Both hooks track lightweight session counters:

- **session-start.sh**: counts context entries injected (stored in `/tmp/agentlog-session-{id}.count`)
- **decision-write.sh**: counts decisions captured (stored in `/tmp/agentlog-decisions/{id}.count`)

These counters persist across invocations within the same Claude Code session and are reported in verbose mode. No data leaves your machine.

## Troubleshooting

### Hooks not firing

- Verify `.claude/settings.json` contains the hook configuration
- Ensure script paths are correct relative to your project root
- Check file permissions: `chmod +x .claude/hooks/session-start.sh .claude/hooks/decision-write.sh`

### "agentlog not found"

- Ensure agentlog is installed: `which agentlog`
- If installed via Homebrew: `brew list agentlog`
- If built from source: ensure `bin/` directory is on your PATH

### Daemon not running

- Start it: `agentlog start`
- Check logs: `cat ~/.agentlog/agentlogd.log`
- Verify socket exists: `ls ~/.agentlog/agentlogd.sock`

### No context appearing

- Verify decisions exist: `agentlog log`
- Verify git state has files: `git diff --name-only` and `git diff --cached --name-only`
- Run the hook manually: `bash .claude/hooks/session-start.sh`
- Use verbose mode to see what the hook is doing

### No decisions captured

- Verify `CLAUDE_SESSION_ID` is set (automatic in Claude Code)
- Verify you are in a git repo
- Check that files were actually modified during the session
- Use verbose mode to see diagnostic output

### Expected silent behavior

Both hooks are designed to be completely silent in normal operation:

- **No stdout** from decision-write.sh (it only writes to the daemon)
- **No stderr** unless `AGENTLOG_VERBOSE=1` is set
- **Exit code 0** in all cases, even on errors (hooks should never block Claude Code)

If agentlog is not installed or the daemon is not running, the hooks exit immediately without error. This is intentional - the hooks should never interfere with the Claude Code experience.
