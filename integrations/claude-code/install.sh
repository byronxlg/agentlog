#!/usr/bin/env bash
#
# install.sh - Set up agentlog hooks for Claude Code in the current project.
#
# Usage:
#   bash <path-to>/install.sh           # Install in current project
#   bash <path-to>/install.sh --global  # Install in global Claude settings
#
# What it does:
#   1. Copies session-start.sh and decision-write.sh into .claude/hooks/
#   2. Creates or patches .claude/settings.json with the hook configuration
#   3. Makes the scripts executable
#
# The install location can be project-level (.claude/ in the current directory)
# or global (~/.claude/) when --global is passed.
#
# Requirements:
#   - agentlog binary on PATH (warns if missing but continues)
#   - jq for JSON manipulation (required)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Parse arguments
global=false
for arg in "$@"; do
    case "$arg" in
        --global) global=true ;;
        --help|-h)
            echo "Usage: bash install.sh [--global]"
            echo ""
            echo "Options:"
            echo "  --global    Install hooks in ~/.claude/ (global) instead of .claude/ (project)"
            echo ""
            echo "Installs agentlog session-start and decision-write hooks for Claude Code."
            exit 0
            ;;
        *)
            echo "Unknown option: $arg" >&2
            echo "Usage: bash install.sh [--global]" >&2
            exit 1
            ;;
    esac
done

# Determine target directory
if $global; then
    target_dir="$HOME/.claude"
    scope="global"
else
    target_dir=".claude"
    scope="project"
fi

# Check for jq
if ! command -v jq &>/dev/null; then
    echo "Error: jq is required but not installed." >&2
    echo "Install it with: brew install jq (macOS) or apt-get install jq (Linux)" >&2
    exit 1
fi

# Warn if agentlog is not on PATH
if ! command -v agentlog &>/dev/null; then
    echo "Warning: agentlog is not on your PATH. The hooks will not work until it is installed."
    echo "Install with: brew install byronxlg/agentlog/agentlog"
    echo ""
fi

# Create hooks directory
hooks_dir="${target_dir}/hooks"
mkdir -p "$hooks_dir"

# Copy hook scripts
cp "$SCRIPT_DIR/session-start.sh" "$hooks_dir/session-start.sh"
cp "$SCRIPT_DIR/decision-write.sh" "$hooks_dir/decision-write.sh"
chmod +x "$hooks_dir/session-start.sh" "$hooks_dir/decision-write.sh"

echo "Copied hooks to ${hooks_dir}/"

# Build the hooks JSON to merge into settings
hooks_json='{
  "hooks": {
    "UserPromptSubmit": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "'"${hooks_dir}"'/session-start.sh"
      }]
    }],
    "Stop": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": "'"${hooks_dir}"'/decision-write.sh"
      }]
    }]
  }
}'

# For project-level, use relative paths
if ! $global; then
    hooks_json='{
  "hooks": {
    "UserPromptSubmit": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/session-start.sh"
      }]
    }],
    "Stop": [{
      "matcher": "",
      "hooks": [{
        "type": "command",
        "command": ".claude/hooks/decision-write.sh"
      }]
    }]
  }
}'
fi

# Create or patch settings.json
settings_file="${target_dir}/settings.json"

if [[ -f "$settings_file" ]]; then
    # Merge hooks into existing settings, preserving other keys.
    # If hooks already exist, the new ones are appended to the arrays.
    existing=$(cat "$settings_file")

    merged=$(echo "$existing" | jq --argjson new "$hooks_json" '
        # For each hook event in $new.hooks, append entries to existing arrays
        .hooks = reduce ($new.hooks | keys[]) as $event (
            (.hooks // {});
            . + { ($event): (.[$event] // []) + $new.hooks[$event] }
        )
    ')

    echo "$merged" | jq '.' > "$settings_file"
    echo "Updated ${settings_file} (merged hooks into existing config)"
else
    echo "$hooks_json" | jq '.' > "$settings_file"
    echo "Created ${settings_file}"
fi

echo ""
echo "agentlog hooks installed (${scope})."
echo ""
echo "Next steps:"
echo "  1. Start the daemon: agentlog start"
echo "  2. Start a Claude Code session - context injection and decision capture are now active"
echo ""
echo "To verify: run 'agentlog log' after a Claude Code session to see captured decisions."
