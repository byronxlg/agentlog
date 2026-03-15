#!/usr/bin/env bash
#
# agentlog-hook.sh - Claude Code hook that logs file modifications as decisions.
#
# This hook is triggered after Edit/Write tool uses in Claude Code. It logs
# which files were modified so you have a record of changes and their context.
#
# Setup:
#   1. Copy to .claude/hooks/agentlog-hook.sh
#   2. chmod +x .claude/hooks/agentlog-hook.sh
#   3. Add to .claude/settings.json:
#      {
#        "hooks": {
#          "PostToolUse": [{
#            "matcher": "Edit|Write",
#            "hooks": [{
#              "type": "command",
#              "command": ".claude/hooks/agentlog-hook.sh"
#            }]
#          }]
#        }
#      }
#
# Environment variables provided by Claude Code:
#   CLAUDE_TOOL_NAME - The tool that was used (e.g., "Edit", "Write")
#   CLAUDE_FILE_PATH - The file that was modified
#   CLAUDE_SESSION_ID - The current Claude Code session ID
#
# Customize the tags, type, and title format below to match your workflow.

set -euo pipefail

# Skip if agentlog is not installed or daemon is not running
if ! command -v agentlog &>/dev/null; then
    exit 0
fi

TOOL_NAME="${CLAUDE_TOOL_NAME:-unknown}"
FILE_PATH="${CLAUDE_FILE_PATH:-unknown}"
SESSION_ID="${CLAUDE_SESSION_ID:-}"

TITLE="File modified via ${TOOL_NAME}: $(basename "$FILE_PATH")"

SESSION_FLAG=""
if [ -n "$SESSION_ID" ]; then
    SESSION_FLAG="--session ${SESSION_ID}"
fi

# Log the modification as a decision entry
# shellcheck disable=SC2086
agentlog write \
    --type decision \
    --title "$TITLE" \
    --body "Claude Code modified ${FILE_PATH} using the ${TOOL_NAME} tool." \
    --tags "claude-code,auto-logged" \
    --files "$FILE_PATH" \
    $SESSION_FLAG \
    2>/dev/null || true
# Errors are silently ignored so the hook never blocks Claude Code
