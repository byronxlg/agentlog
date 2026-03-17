# Claude Code Integration

Two hooks for integrating agentlog with Claude Code:

- **session-start.sh** - Injects relevant past decisions into the conversation at session start
- **decision-write.sh** - Automatically captures decisions when files are changed during a session

## Hooks

### session-start.sh (context injection)

Runs on `UserPromptSubmit` events. On the first prompt of each session, it detects your current working set of files from git state and queries the agentlog daemon for relevant past decisions. The output is markdown that Claude Code injects into the conversation context.

### decision-write.sh (automatic capture)

Runs on `Stop` events (after each Claude Code response). It detects files modified during the current turn by comparing git state against a per-session snapshot and logs new changes as decisions via `agentlog write`. All operations are silent - no user-visible output.

The hook tracks state across turns using `CLAUDE_SESSION_ID`:
- Maintains a file snapshot per session to detect only newly changed files each turn
- Reuses the same agentlog session across all turns in a Claude Code session
- Skips turns where no new file changes occurred

## Installation

### Automated (recommended)

Run the install script from your project root:

```bash
bash integrations/claude-code/install.sh
```

This copies both hook scripts to `.claude/hooks/`, creates or patches `.claude/settings.json` with the hook configuration, and makes the scripts executable.

For global installation (applies to all projects):

```bash
bash integrations/claude-code/install.sh --global
```

Requires `jq` for JSON manipulation. Install with `brew install jq` (macOS) or `apt-get install jq` (Linux).

### Manual

1. Ensure the agentlog daemon is running (`agentlog start`).

2. Add the hooks to your Claude Code settings. Edit `.claude/settings.json` (project-level) or `~/.claude/settings.json` (global):

```json
{
  "hooks": {
    "UserPromptSubmit": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "integrations/claude-code/session-start.sh"
      }]
    }],
    "Stop": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "integrations/claude-code/decision-write.sh"
      }]
    }]
  }
}
```

3. Make the scripts executable:

```bash
chmod +x integrations/claude-code/session-start.sh integrations/claude-code/decision-write.sh
```

## Environment variables

### session-start.sh

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_SESSION_ID` | (set by Claude Code) | Used to run the query only once per session. If set, a marker file is written to `/tmp` so subsequent prompts skip the daemon query. |
| `AGENTLOG_LIMIT` | `10` | Maximum number of decision entries to retrieve |
| `AGENTLOG_TOPIC` | repo or directory name | Override the fallback topic when no files are detected |

### decision-write.sh

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_SESSION_ID` | (set by Claude Code) | Required. Used to track file changes across turns and maintain a consistent agentlog session. The hook exits silently if this is not set. |
| `AGENTLOG_TAGS` | `claude-code` | Comma-separated tags added to every captured decision |

## Customization

- **Limit context results:** Set `AGENTLOG_LIMIT=5` to reduce the amount of injected context.
- **Force topic search:** Set `AGENTLOG_TOPIC="my-project"` to always search by topic instead of file paths.
- **Custom tags:** Set `AGENTLOG_TAGS="claude-code,my-project"` to tag captured decisions.
- **File detection:** Both scripts automatically use git state. The session-start hook falls back to searching by directory name if not in a git repo. The decision-write hook requires a git repo.

## Troubleshooting

- **No context appearing:** Verify the daemon is running with `agentlog log`. Check that decisions have been logged (output should show entries, not an error).
- **Decisions not being captured:** Ensure `CLAUDE_SESSION_ID` is available (set automatically by Claude Code). Check that the daemon is running and that file changes are being made in a git repo.
- **Hook not triggering:** Confirm `.claude/settings.json` is valid JSON and the script paths are correct relative to your project root.
- **Permission denied:** Run `chmod +x integrations/claude-code/session-start.sh integrations/claude-code/decision-write.sh`.
- **agentlog not found:** Ensure `agentlog` is in your PATH. Both hooks silently exit if the binary is missing.
- **Debug session-start:** Run the script directly from your project root to see its output: `bash integrations/claude-code/session-start.sh`.
- **Debug decision-write:** Check for snapshot files in `/tmp/agentlog-decisions/` to verify state tracking is working.

## End-to-end walkthrough

This walkthrough goes from a fresh install to a working Claude Code session with context injection and decision capture.

### 1. Install agentlog

```bash
brew install byronxlg/agentlog/agentlog
```

Or build from source:

```bash
git clone https://github.com/byronxlg/agentlog.git
cd agentlog && make build
export PATH="$PWD/bin:$PATH"
```

### 2. Start the daemon

```bash
agentlog start
```

Verify it is running:

```bash
agentlog log
# Expected: "no entries found" (empty log is fine)
```

### 3. Set up hooks in your project

Navigate to the project where you use Claude Code, then run the install script:

```bash
cd /path/to/your-project
bash /path/to/agentlog/integrations/claude-code/install.sh
```

This creates `.claude/hooks/session-start.sh`, `.claude/hooks/decision-write.sh`, and patches `.claude/settings.json`.

### 4. Seed some decisions (optional)

If this is a new install, there are no past decisions to inject yet. You can seed a few to verify context injection works:

```bash
agentlog write --type decision \
  --title "Use React for the frontend" \
  --body "Considered Vue and Svelte. React chosen for team familiarity." \
  --tags "architecture,frontend" \
  --files "src/App.tsx"
```

### 5. Start a Claude Code session

```bash
claude
```

On your first prompt, the session-start hook runs and queries the daemon. If there are decisions related to your working files, they appear as context in the conversation.

### 6. Verify context injection

After sending your first prompt, check that Claude received context. If you seeded decisions in step 4 and have `src/App.tsx` in your git working set, the decision about React should appear in the conversation context.

You can also run the hook manually to see its output:

```bash
bash .claude/hooks/session-start.sh
```

### 7. Verify decision capture

After Claude makes some file changes in the session, the decision-write hook runs automatically. Verify captured decisions:

```bash
agentlog log --tag claude-code
```

You should see entries for the files Claude modified, tagged with `claude-code`.

### 8. Query decisions later

```bash
# Full-text search
agentlog query "React"

# Decisions affecting a specific file
agentlog blame src/App.tsx

# Context API (same as what the hook uses)
agentlog context --files src/App.tsx --limit 5
```
