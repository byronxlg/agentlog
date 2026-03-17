#!/usr/bin/env bash
#
# session-start.sh - Claude Code hook that injects past decisions into sessions.
#
# This hook runs on UserPromptSubmit and queries the agentlog daemon for
# decisions relevant to your current working set of files. The output is
# markdown that Claude Code prepends to the conversation context.
#
# Setup:
#   1. Copy this script somewhere accessible (e.g., your project root or
#      .claude/hooks/).
#   2. chmod +x session-start.sh
#   3. Add to .claude/settings.json:
#      {
#        "hooks": {
#          "UserPromptSubmit": [{
#            "matcher": "",
#            "hooks": [{
#              "type": "command",
#              "command": "integrations/claude-code/session-start.sh"
#            }]
#          }]
#        }
#      }
#
# Environment variables:
#   CLAUDE_SESSION_ID  - Set by Claude Code; used to run the query only once per session
#   AGENTLOG_LIMIT     - Max entries to retrieve (default: 10)
#   AGENTLOG_TOPIC     - Override the fallback topic (default: repo or dir name)
#
# Customization:
#   - Adjust AGENTLOG_LIMIT to control how many decisions are returned
#   - Set AGENTLOG_TOPIC to always search by a specific topic instead of files
#   - The script auto-detects files from git state; no configuration needed
#
# The script exits 0 silently (no output) when:
#   - agentlog is not installed
#   - The daemon is not running
#   - No relevant decisions are found

set -euo pipefail

# Exit silently if agentlog is not installed
if ! command -v agentlog &>/dev/null; then
    exit 0
fi

LIMIT="${AGENTLOG_LIMIT:-10}"

# Run only once per session. If CLAUDE_SESSION_ID is set, use a marker file
# to skip subsequent invocations within the same session.
if [[ -n "${CLAUDE_SESSION_ID:-}" ]]; then
    marker="/tmp/agentlog-session-${CLAUDE_SESSION_ID}"
    if [[ -f "$marker" ]]; then
        exit 0
    fi
fi

# Collect working set files from git state.
# Each source is optional - we handle non-git directories gracefully.
files=()

if git rev-parse --is-inside-work-tree &>/dev/null; then
    # Staged files
    while IFS= read -r f; do
        [[ -n "$f" ]] && files+=("$f")
    done < <(git diff --cached --name-only 2>/dev/null)

    # Unstaged modified files
    while IFS= read -r f; do
        [[ -n "$f" ]] && files+=("$f")
    done < <(git diff --name-only 2>/dev/null)

    # Files from the last commit
    while IFS= read -r f; do
        [[ -n "$f" ]] && files+=("$f")
    done < <(git diff --name-only HEAD~1 HEAD 2>/dev/null)
fi

# Deduplicate the file list while preserving order
if (( ${#files[@]} > 0 )); then
    declare -A seen
    unique_files=()
    for f in "${files[@]}"; do
        if [[ -z "${seen[$f]:-}" ]]; then
            seen[$f]=1
            unique_files+=("$f")
        fi
    done
    files=("${unique_files[@]}")
fi

# Build the agentlog context command
cmd=(agentlog context --limit "$LIMIT")

if (( ${#files[@]} > 0 )); then
    for f in "${files[@]}"; do
        cmd+=(--files "$f")
    done
else
    # Fallback: use repo name or directory name as topic
    topic="${AGENTLOG_TOPIC:-}"
    if [[ -z "$topic" ]]; then
        if git rev-parse --is-inside-work-tree &>/dev/null; then
            topic="$(basename "$(git rev-parse --show-toplevel 2>/dev/null)")"
        else
            topic="$(basename "$PWD")"
        fi
    fi
    cmd+=(--topic "$topic")
fi

# Query the daemon, capturing output. Exit silently on any failure
# (daemon not running, socket missing, etc.)
output=$("${cmd[@]}" 2>/dev/null) || exit 0

# Suppress the "no results" message - it adds noise to the context
if [[ -z "$output" || "$output" == "No relevant decisions found." ]]; then
    exit 0
fi

# Mark this session as having received context
if [[ -n "${CLAUDE_SESSION_ID:-}" ]]; then
    touch "$marker"
fi

printf '%s\n' "$output"
