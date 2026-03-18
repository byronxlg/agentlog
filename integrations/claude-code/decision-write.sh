#!/usr/bin/env bash
#
# decision-write.sh - Claude Code hook that captures agent decisions.
#
# This hook runs on Stop events (after each Claude Code response). It detects
# files modified during the current turn by comparing git state against a
# per-session snapshot, then logs new changes as decisions via agentlog write.
#
# Setup:
#   1. Copy this script somewhere accessible (e.g., your project root or
#      .claude/hooks/).
#   2. chmod +x decision-write.sh
#   3. Add to .claude/settings.json:
#      {
#        "hooks": {
#          "Stop": [{
#            "matcher": "",
#            "hooks": [{
#              "type": "command",
#              "command": "integrations/claude-code/decision-write.sh"
#            }]
#          }]
#        }
#      }
#
# Environment variables:
#   CLAUDE_SESSION_ID  - Set by Claude Code; used to track state across turns
#   AGENTLOG_TAGS      - Comma-separated tags to add to every decision (default: "claude-code")
#   AGENTLOG_VERBOSE   - Set to 1 to print diagnostic output to stderr at each step
#   AGENTLOG_DRY_RUN   - Set to 1 to skip the agentlog write call and print what would be written
#
# The script exits 0 silently in all cases:
#   - agentlog is not installed
#   - No CLAUDE_SESSION_ID (cannot track cross-turn state)
#   - No git repo found
#   - No new file changes detected this turn
#   - The daemon is not running
#   - Any other error

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

# Require CLAUDE_SESSION_ID to track state across turns.
# Without it we cannot distinguish new changes from previously seen ones.
session_id="${CLAUDE_SESSION_ID:-}"
if [[ -z "$session_id" ]]; then
    verbose "CLAUDE_SESSION_ID not set, exiting"
    exit 0
fi

verbose "session_id=$session_id"

# Exit silently if not in a git repo
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    verbose "not in a git repo, exiting"
    exit 0
fi

verbose "git repo detected at $(git rev-parse --show-toplevel 2>/dev/null)"

# Collect currently modified files from git state
files=()

# Staged files
while IFS= read -r f; do
    [[ -n "$f" ]] && files+=("$f")
done < <(git diff --cached --name-only 2>/dev/null)

# Unstaged modified files
while IFS= read -r f; do
    [[ -n "$f" ]] && files+=("$f")
done < <(git diff --name-only 2>/dev/null)

# Untracked files
while IFS= read -r f; do
    [[ -n "$f" ]] && files+=("$f")
done < <(git ls-files --others --exclude-standard 2>/dev/null)

verbose "detected ${#files[@]} file(s) from git state"

# Deduplicate
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

# Compare with previous snapshot to find newly changed files
snapshot_dir="/tmp/agentlog-decisions"
snapshot_file="${snapshot_dir}/${session_id}.files"
mkdir -p "$snapshot_dir"

# Load previous snapshot into a set
declare -A prev_set
if [[ -f "$snapshot_file" ]]; then
    while IFS= read -r f; do
        [[ -n "$f" ]] && prev_set[$f]=1
    done < "$snapshot_file"
    verbose "loaded ${#prev_set[@]} file(s) from previous snapshot"
else
    verbose "no previous snapshot found"
fi

# Find files not in previous snapshot
new_files=()
for f in "${files[@]+"${files[@]}"}"; do
    if [[ -z "${prev_set[$f]:-}" ]]; then
        new_files+=("$f")
    fi
done

verbose "found ${#new_files[@]} new file(s) this turn"

# Save current snapshot (all files, not just new ones)
if (( ${#files[@]} > 0 )); then
    printf '%s\n' "${files[@]}" > "$snapshot_file"
else
    # No files at all - clear the snapshot
    rm -f "$snapshot_file"
fi

# Exit if no new changes this turn
if (( ${#new_files[@]} == 0 )); then
    verbose "no new changes, exiting"
    exit 0
fi

# Build title from newly changed files
if (( ${#new_files[@]} <= 3 )); then
    title="Modified ${new_files[*]}"
else
    title="Modified ${new_files[0]} and $((${#new_files[@]} - 1)) other files"
fi

# Build comma-separated file list for --files flag
file_csv=""
for f in "${new_files[@]}"; do
    if [[ -n "$file_csv" ]]; then
        file_csv="${file_csv},${f}"
    else
        file_csv="$f"
    fi
done

# Tags default to "claude-code" but can be overridden
tags="${AGENTLOG_TAGS:-claude-code}"

# Get or create an agentlog session for this Claude session
session_file="${snapshot_dir}/${session_id}.session"
agentlog_session=""
if [[ -f "$session_file" ]]; then
    agentlog_session=$(cat "$session_file")
fi

# Build the agentlog write command
cmd=(agentlog write --type decision --title "$title" --files "$file_csv" --tags "$tags")
if [[ -n "$agentlog_session" ]]; then
    cmd+=(--session "$agentlog_session")
fi

verbose "command: ${cmd[*]}"

# In dry-run mode, print what would be done and exit
if [[ "$DRY_RUN" == "1" ]]; then
    printf '[agentlog] dry-run: would execute: %s\n' "${cmd[*]}" >&2
    exit 0
fi

# Call agentlog write. Capture stderr (contains session ID on first write).
stderr_tmp="${snapshot_dir}/${session_id}.stderr"
"${cmd[@]}" 2>"$stderr_tmp" || true

# Save agentlog session ID from first write for reuse
if [[ -z "$agentlog_session" ]] && [[ -s "$stderr_tmp" ]]; then
    cat "$stderr_tmp" > "$session_file"
fi

verbose "decision written"

rm -f "$stderr_tmp"
exit 0
