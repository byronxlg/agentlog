#!/usr/bin/env bash
#
# install_test.sh - Tests for the Claude Code install script.
#
# Run: bash integrations/claude-code/install_test.sh
#
# Each test sets up an isolated temp directory, runs the install script,
# and verifies the expected output.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_SCRIPT="${SCRIPT_DIR}/install.sh"

TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# -- Test helpers --

pass() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    printf "  PASS: %s\n" "$1"
}

fail() {
    TESTS_FAILED=$((TESTS_FAILED + 1))
    printf "  FAIL: %s\n" "$1"
    if [[ -n "${2:-}" ]]; then
        printf "        %s\n" "$2"
    fi
}

run_test() {
    TESTS_RUN=$((TESTS_RUN + 1))
    printf "Running: %s\n" "$1"
    "$1"
}

TMPDIRS=()
cleanup() {
    for d in "${TMPDIRS[@]}"; do
        rm -rf "$d"
    done
}
trap cleanup EXIT

make_tmpdir() {
    local d
    d="$(mktemp -d)"
    TMPDIRS+=("$d")
    echo "$d"
}

# -- Tests --

test_fails_without_jq() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Run install with a PATH that has no jq
    local output exit_code
    output=$(cd "$tmpdir" && PATH="/usr/bin:/bin" bash "$INSTALL_SCRIPT" 2>&1) && exit_code=$? || exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        pass "exits non-zero when jq is missing"
    else
        fail "exits non-zero when jq is missing" "got exit code $exit_code"
    fi

    if echo "$output" | grep -qi "jq"; then
        pass "mentions jq in error message"
    else
        fail "mentions jq in error message" "output: $output"
    fi
}

test_creates_hooks_and_settings_in_new_project() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a mock agentlog so the warning is suppressed
    local mockdir
    mockdir="$(make_tmpdir)"
    printf '#!/usr/bin/env bash\nexit 0\n' > "$mockdir/agentlog"
    chmod +x "$mockdir/agentlog"

    local output
    output=$(cd "$tmpdir" && PATH="$mockdir:$(command -v jq | xargs dirname):/usr/bin:/bin" bash "$INSTALL_SCRIPT" 2>&1)

    # Check hooks were copied
    if [[ -f "$tmpdir/.claude/hooks/session-start.sh" ]]; then
        pass "copies session-start.sh to .claude/hooks/"
    else
        fail "copies session-start.sh to .claude/hooks/" "file not found"
    fi

    if [[ -f "$tmpdir/.claude/hooks/decision-write.sh" ]]; then
        pass "copies decision-write.sh to .claude/hooks/"
    else
        fail "copies decision-write.sh to .claude/hooks/" "file not found"
    fi

    # Check scripts are executable
    if [[ -x "$tmpdir/.claude/hooks/session-start.sh" ]]; then
        pass "session-start.sh is executable"
    else
        fail "session-start.sh is executable" "not executable"
    fi

    if [[ -x "$tmpdir/.claude/hooks/decision-write.sh" ]]; then
        pass "decision-write.sh is executable"
    else
        fail "decision-write.sh is executable" "not executable"
    fi

    # Check settings.json was created with correct structure
    if [[ -f "$tmpdir/.claude/settings.json" ]]; then
        pass "creates settings.json"
    else
        fail "creates settings.json" "file not found"
        return
    fi

    local settings
    settings=$(cat "$tmpdir/.claude/settings.json")

    if echo "$settings" | jq -e '.hooks.UserPromptSubmit' &>/dev/null; then
        pass "settings.json contains UserPromptSubmit hook"
    else
        fail "settings.json contains UserPromptSubmit hook" "missing from: $settings"
    fi

    if echo "$settings" | jq -e '.hooks.Stop' &>/dev/null; then
        pass "settings.json contains Stop hook"
    else
        fail "settings.json contains Stop hook" "missing from: $settings"
    fi

    # Check relative paths in project mode
    if echo "$settings" | jq -r '.hooks.UserPromptSubmit[0].hooks[0].command' | grep -q "^\.claude/hooks/"; then
        pass "uses relative paths for project-level install"
    else
        fail "uses relative paths for project-level install" "settings: $settings"
    fi
}

test_merges_into_existing_settings() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create existing settings with a pre-existing key
    mkdir -p "$tmpdir/.claude"
    cat > "$tmpdir/.claude/settings.json" <<'JSON'
{
  "permissions": {
    "allow": ["Bash(git *)"]
  }
}
JSON

    local mockdir
    mockdir="$(make_tmpdir)"
    printf '#!/usr/bin/env bash\nexit 0\n' > "$mockdir/agentlog"
    chmod +x "$mockdir/agentlog"

    output=$(cd "$tmpdir" && PATH="$mockdir:$(command -v jq | xargs dirname):/usr/bin:/bin" bash "$INSTALL_SCRIPT" 2>&1)

    local settings
    settings=$(cat "$tmpdir/.claude/settings.json")

    # Existing keys should be preserved
    if echo "$settings" | jq -e '.permissions.allow' &>/dev/null; then
        pass "preserves existing settings keys"
    else
        fail "preserves existing settings keys" "settings: $settings"
    fi

    # Hooks should be added
    if echo "$settings" | jq -e '.hooks.UserPromptSubmit' &>/dev/null; then
        pass "adds hooks to existing settings"
    else
        fail "adds hooks to existing settings" "settings: $settings"
    fi
}

test_warns_when_agentlog_not_on_path() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Run without agentlog on PATH
    local output
    output=$(cd "$tmpdir" && PATH="$(command -v jq | xargs dirname):/usr/bin:/bin" bash "$INSTALL_SCRIPT" 2>&1)

    if echo "$output" | grep -qi "warning.*agentlog"; then
        pass "warns when agentlog is not on PATH"
    else
        fail "warns when agentlog is not on PATH" "output: $output"
    fi

    # Should still complete (exit 0)
    if [[ -f "$tmpdir/.claude/settings.json" ]]; then
        pass "completes install even without agentlog on PATH"
    else
        fail "completes install even without agentlog on PATH" "settings.json not created"
    fi
}

test_help_flag() {
    local output exit_code
    output=$(bash "$INSTALL_SCRIPT" --help 2>&1) && exit_code=$? || exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "--help exits 0"
    else
        fail "--help exits 0" "got exit code $exit_code"
    fi

    if echo "$output" | grep -q "Usage"; then
        pass "--help shows usage"
    else
        fail "--help shows usage" "output: $output"
    fi
}

test_unknown_flag_fails() {
    local output exit_code
    output=$(bash "$INSTALL_SCRIPT" --unknown 2>&1) && exit_code=$? || exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        pass "unknown flag exits non-zero"
    else
        fail "unknown flag exits non-zero" "got exit code $exit_code"
    fi
}

# -- Run all tests --

printf "=== install.sh tests ===\n\n"

run_test test_fails_without_jq
run_test test_creates_hooks_and_settings_in_new_project
run_test test_merges_into_existing_settings
run_test test_warns_when_agentlog_not_on_path
run_test test_help_flag
run_test test_unknown_flag_fails

printf "\n=== Results: %d tests, %d passed, %d failed ===\n" "$TESTS_RUN" "$TESTS_PASSED" "$TESTS_FAILED"

if [[ $TESTS_FAILED -gt 0 ]]; then
    exit 1
fi
