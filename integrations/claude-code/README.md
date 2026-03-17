# Claude Code Integration

Session-start hook that automatically injects relevant past decisions from agentlog into your Claude Code conversations.

## What it does

When you start a Claude Code session, the `session-start.sh` hook:

1. Detects your current working set of files from git state (staged, modified, and recently committed files)
2. Queries the agentlog daemon for decisions related to those files
3. Outputs formatted markdown context that Claude Code injects into the conversation

This gives Claude awareness of past architectural decisions, design rationale, and other logged context relevant to the files you are working on.

## Installation

1. Ensure the agentlog daemon is running (`agentlog start`).

2. Add the hook to your Claude Code settings. Edit `.claude/settings.json` (project-level) or `~/.claude/settings.json` (global):

```json
{
  "hooks": {
    "UserPromptSubmit": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "integrations/claude-code/session-start.sh"
      }]
    }]
  }
}
```

3. Make the script executable:

```bash
chmod +x integrations/claude-code/session-start.sh
```

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_SESSION_ID` | (set by Claude Code) | Used to run the query only once per session. If set, a marker file is written to `/tmp` so subsequent prompts skip the daemon query. |
| `AGENTLOG_LIMIT` | `10` | Maximum number of decision entries to retrieve |
| `AGENTLOG_TOPIC` | repo or directory name | Override the fallback topic when no files are detected |

## Customization

- **Limit results:** Set `AGENTLOG_LIMIT=5` to reduce the amount of injected context.
- **Force topic search:** Set `AGENTLOG_TOPIC="my-project"` to always search by topic instead of file paths.
- **File detection:** The script automatically uses git state. If you are not in a git repo, it falls back to searching by the current directory name.

## Troubleshooting

- **No context appearing:** Verify the daemon is running with `agentlog log`. Check that decisions have been logged (output should show entries, not an error).
- **Hook not triggering:** Confirm `.claude/settings.json` is valid JSON and the script path is correct relative to your project root.
- **Permission denied:** Run `chmod +x integrations/claude-code/session-start.sh`.
- **agentlog not found:** Ensure `agentlog` is in your PATH. The hook silently exits if the binary is missing.
- **Debug output:** Run the script directly from your project root to see its output: `bash integrations/claude-code/session-start.sh`.
