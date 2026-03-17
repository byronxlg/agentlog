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
#
# The script exits 0 silently in all cases:
#   - agentlog is not installed
#   - No CLAUDE_SESSION_ID (cannot track cross-turn state)
#   - No git repo found
#   - No new file changes detected this turn
#   - The daemon is not running
#   - Any other error

set -euo pipefail

# Exit silently if agentlog is not installed
if ! command -v agentlog &>/dev/null; then
    exit 0
fi

# Require CLAUDE_SESSION_ID to track state across turns.
# Without it we cannot distinguish new changes from previously seen ones.
session_id="${CLAUDE_SESSION_ID:-}"
if [[ -z "$session_id" ]]; then
    exit 0
fi

# Exit silently if not in a git repo
if ! git rev-parse --is-inside-work-tree &>/dev/null; then
    exit 0
fi

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
fi

# Find files not in previous snapshot
new_files=()
for f in "${files[@]+"${files[@]}"}"; do
    if [[ -z "${prev_set[$f]:-}" ]]; then
        new_files+=("$f")
    fi
done

# Save current snapshot (all files, not just new ones)
if (( ${#files[@]} > 0 )); then
    printf '%s\n' "${files[@]}" > "$snapshot_file"
else
    # No files at all - clear the snapshot
    rm -f "$snapshot_file"
fi

# Exit if no new changes this turn
if (( ${#new_files[@]} == 0 )); then
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

# Call agentlog write. Capture stderr (contains session ID on first write).
stderr_tmp="${snapshot_dir}/${session_id}.stderr"
"${cmd[@]}" 2>"$stderr_tmp" || true

# Save agentlog session ID from first write for reuse
if [[ -z "$agentlog_session" ]] && [[ -s "$stderr_tmp" ]]; then
    cat "$stderr_tmp" > "$session_file"
fi

rm -f "$stderr_tmp"
exit 0
