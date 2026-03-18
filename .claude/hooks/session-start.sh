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
#   AGENTLOG_VERBOSE   - Set to 1 to print diagnostic output to stderr at each step
#   AGENTLOG_DRY_RUN   - Set to 1 to skip the agentlog context call and print what would be queried
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

VERBOSE="${AGENTLOG_VERBOSE:-0}"
DRY_RUN="${AGENTLOG_DRY_RUN:-0}"

verbose() {
    if [[ "$VERBOSE" == "1" ]]; then
        printf '[agentlog] %s\n' "$1" >&2
    fi
}

# Exit silently if agentlog is not installed
if ! command -v agentlog &>/dev/null; then
    verbose "agentlog not found on PATH, exiting"
    exit 0
fi

LIMIT="${AGENTLOG_LIMIT:-10}"

# Run only once per session. If CLAUDE_SESSION_ID is set, use a marker file
# to skip subsequent invocations within the same session.
if [[ -n "${CLAUDE_SESSION_ID:-}" ]]; then
    marker="/tmp/agentlog-session-${CLAUDE_SESSION_ID}"
    if [[ -f "$marker" ]]; then
        verbose "session already initialized (marker exists), exiting"
        exit 0
    fi
fi

verbose "limit=$LIMIT"

# Collect working set files from git state.
# Each source is optional - we handle non-git directories gracefully.
files=()

if git rev-parse --is-inside-work-tree &>/dev/null; then
    verbose "git repo detected at $(git rev-parse --show-toplevel 2>/dev/null)"

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
else
    verbose "not in a git repo, will use topic fallback"
fi

verbose "detected ${#files[@]} file(s) from git state"

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
    verbose "after dedup: ${#files[@]} unique file(s)"
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
    verbose "using topic fallback: $topic"
fi

verbose "command: ${cmd[*]}"

# In dry-run mode, print what would be done and exit
if [[ "$DRY_RUN" == "1" ]]; then
    printf '[agentlog] dry-run: would execute: %s\n' "${cmd[*]}" >&2
    exit 0
fi

# Query the daemon, capturing output. Exit silently on any failure
# (daemon not running, socket missing, etc.)
output=$("${cmd[@]}" 2>/dev/null) || exit 0

# Suppress the "no results" message - it adds noise to the context
if [[ -z "$output" || "$output" == "No relevant decisions found." ]]; then
    verbose "no relevant decisions found"
    exit 0
fi

verbose "injecting ${output%%$'\n'*}..."

# Mark this session as having received context
if [[ -n "${CLAUDE_SESSION_ID:-}" ]]; then
    touch "$marker"
fi

printf '%s\n' "$output"
