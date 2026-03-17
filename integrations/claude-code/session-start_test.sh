#!/usr/bin/env bash
#
# session-start_test.sh - Tests for the Claude Code session-start hook.
#
# Run: bash integrations/claude-code/session-start_test.sh
#
# Each test function sets up an isolated environment (temp dirs, mock PATHs,
# temp git repos) and asserts expected behavior. Tests are self-contained
# and clean up after themselves.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK_SCRIPT="${SCRIPT_DIR}/session-start.sh"

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

# Create a temporary directory that is cleaned up on exit
make_tmpdir() {
    local d
    d="$(mktemp -d)"
    # Register cleanup. We accumulate dirs and clean them all at the end.
    TMPDIRS+=("$d")
    echo "$d"
}

TMPDIRS=()
cleanup() {
    for d in "${TMPDIRS[@]}"; do
        rm -rf "$d"
    done
}
trap cleanup EXIT

# -- Tests --

test_exits_zero_when_agentlog_not_installed() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Run the hook with a PATH that contains no agentlog binary.
    # The script should exit 0 silently since agentlog is not found.
    local output exit_code
    output=$(PATH="$tmpdir" bash "$HOOK_SCRIPT" 2>&1) && exit_code=$? || exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 when agentlog is not installed"
    else
        fail "exits 0 when agentlog is not installed" "got exit code $exit_code"
    fi

    if [[ -z "$output" ]]; then
        pass "produces no output when agentlog is not installed"
    else
        fail "produces no output when agentlog is not installed" "got output: $output"
    fi
}

test_exits_zero_when_daemon_not_running() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a mock agentlog that simulates daemon not running (exit 1)
    cat > "$tmpdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
if [[ "${1:-}" == "context" ]]; then
    echo "daemon is not running" >&2
    exit 1
fi
exit 0
MOCK
    chmod +x "$tmpdir/agentlog"

    output=$(PATH="$tmpdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)
    exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 when daemon is not running"
    else
        fail "exits 0 when daemon is not running" "got exit code $exit_code"
    fi

    if [[ -z "$output" ]]; then
        pass "produces no output when daemon is not running"
    else
        fail "produces no output when daemon is not running" "got output: $output"
    fi
}

test_identifies_git_staged_files() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a git repo with a staged file
    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "initial" > "$tmpdir/base.txt"
    git -C "$tmpdir" add base.txt
    git -C "$tmpdir" commit -q -m "initial"

    echo "staged content" > "$tmpdir/staged.txt"
    git -C "$tmpdir" add staged.txt

    # Create a mock agentlog that captures its arguments
    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
# Write all arguments to a file for inspection
echo "$@" > "${AGENTLOG_TEST_ARGFILE}"
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local argfile="$tmpdir/captured_args.txt"
    output=$(cd "$tmpdir" && AGENTLOG_TEST_ARGFILE="$argfile" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--files staged.txt"; then
            pass "identifies git staged files"
        else
            fail "identifies git staged files" "args were: $args"
        fi
    else
        fail "identifies git staged files" "agentlog was not called"
    fi
}

test_identifies_git_modified_files() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a git repo with a modified (unstaged) file
    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "initial" > "$tmpdir/tracked.txt"
    git -C "$tmpdir" add tracked.txt
    git -C "$tmpdir" commit -q -m "initial"

    echo "modified content" > "$tmpdir/tracked.txt"

    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "$@" > "${AGENTLOG_TEST_ARGFILE}"
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local argfile="$tmpdir/captured_args.txt"
    output=$(cd "$tmpdir" && AGENTLOG_TEST_ARGFILE="$argfile" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--files tracked.txt"; then
            pass "identifies git modified files"
        else
            fail "identifies git modified files" "args were: $args"
        fi
    else
        fail "identifies git modified files" "agentlog was not called"
    fi
}

test_deduplicates_files() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a git repo where the same file appears in both staged and last-commit
    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "v1" > "$tmpdir/shared.txt"
    git -C "$tmpdir" add shared.txt
    git -C "$tmpdir" commit -q -m "initial"

    # Modify and stage the same file (it will appear in both diff --cached and
    # diff --name-only HEAD~1 HEAD)
    echo "v2" > "$tmpdir/shared.txt"
    git -C "$tmpdir" add shared.txt

    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "$@" > "${AGENTLOG_TEST_ARGFILE}"
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local argfile="$tmpdir/captured_args.txt"
    output=$(cd "$tmpdir" && AGENTLOG_TEST_ARGFILE="$argfile" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        # Count occurrences of "--files shared.txt" - should appear exactly once
        local count
        count=$(echo "$args" | grep -o -- "--files shared.txt" | wc -l | tr -d ' ')
        if [[ "$count" -eq 1 ]]; then
            pass "deduplicates files appearing in multiple git sources"
        else
            fail "deduplicates files appearing in multiple git sources" "found $count occurrences in: $args"
        fi
    else
        fail "deduplicates files appearing in multiple git sources" "agentlog was not called"
    fi
}

test_builds_correct_context_command_with_files() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a git repo with multiple files in different states
    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "a" > "$tmpdir/file_a.txt"
    echo "b" > "$tmpdir/file_b.txt"
    git -C "$tmpdir" add file_a.txt file_b.txt
    git -C "$tmpdir" commit -q -m "initial"

    # Stage one new file, modify another
    echo "c" > "$tmpdir/file_c.txt"
    git -C "$tmpdir" add file_c.txt
    echo "b modified" > "$tmpdir/file_b.txt"

    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "$@" > "${AGENTLOG_TEST_ARGFILE}"
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local argfile="$tmpdir/captured_args.txt"
    output=$(cd "$tmpdir" && AGENTLOG_TEST_ARGFILE="$argfile" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        # Should start with "context --limit 10"
        if echo "$args" | grep -q "^context --limit 10"; then
            pass "builds correct context command prefix"
        else
            fail "builds correct context command prefix" "args were: $args"
        fi
        # Should contain --files for both file_c.txt (staged) and file_b.txt (modified)
        if echo "$args" | grep -q -- "--files file_c.txt" && echo "$args" | grep -q -- "--files file_b.txt"; then
            pass "builds correct context command with --files flags for staged and modified files"
        else
            fail "builds correct context command with --files flags for staged and modified files" "args were: $args"
        fi
    else
        fail "builds correct context command prefix" "agentlog was not called"
        fail "builds correct context command with --files flags for staged and modified files" "agentlog was not called"
    fi
}

test_no_output_when_no_relevant_decisions() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a git repo with a modified file
    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "a" > "$tmpdir/file.txt"
    git -C "$tmpdir" add file.txt
    git -C "$tmpdir" commit -q -m "initial"
    echo "modified" > "$tmpdir/file.txt"

    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    output=$(cd "$tmpdir" && PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)
    exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 when no relevant decisions found"
    else
        fail "exits 0 when no relevant decisions found" "got exit code $exit_code"
    fi

    if [[ -z "$output" ]]; then
        pass "produces no output when no relevant decisions found"
    else
        fail "produces no output when no relevant decisions found" "got output: $output"
    fi
}

test_works_when_not_in_git_repo() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # tmpdir is not a git repo - the script should fall back to --topic
    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "$@" > "${AGENTLOG_TEST_ARGFILE}"
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local argfile="$tmpdir/captured_args.txt"
    output=$(cd "$tmpdir" && AGENTLOG_TEST_ARGFILE="$argfile" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)
    exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 when not in a git repo"
    else
        fail "exits 0 when not in a git repo" "got exit code $exit_code"
    fi

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        local dirname
        dirname="$(basename "$tmpdir")"
        if echo "$args" | grep -q -- "--topic $dirname"; then
            pass "falls back to --topic with directory name when not in git repo"
        else
            fail "falls back to --topic with directory name when not in git repo" "args were: $args"
        fi
    else
        fail "falls back to --topic with directory name when not in git repo" "agentlog was not called"
    fi
}

test_outputs_context_when_decisions_found() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a git repo with a modified file
    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "a" > "$tmpdir/file.txt"
    git -C "$tmpdir" add file.txt
    git -C "$tmpdir" commit -q -m "initial"
    echo "modified" > "$tmpdir/file.txt"

    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
cat <<'CONTEXT'
# Relevant decisions

## [decision] Use structured logging (2025-01-15T10:30:00Z)
Switched from fmt.Println to slog for all log output.
Files: internal/daemon/server.go
Tags: architecture
CONTEXT
MOCK
    chmod +x "$mockdir/agentlog"

    output=$(cd "$tmpdir" && PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)
    exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 when decisions are found"
    else
        fail "exits 0 when decisions are found" "got exit code $exit_code"
    fi

    if echo "$output" | grep -q "# Relevant decisions"; then
        pass "outputs context markdown when decisions are found"
    else
        fail "outputs context markdown when decisions are found" "got output: $output"
    fi
}

test_respects_agentlog_limit_env_var() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "a" > "$tmpdir/file.txt"
    git -C "$tmpdir" add file.txt
    git -C "$tmpdir" commit -q -m "initial"
    echo "modified" > "$tmpdir/file.txt"

    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "$@" > "${AGENTLOG_TEST_ARGFILE}"
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local argfile="$tmpdir/captured_args.txt"
    output=$(cd "$tmpdir" && AGENTLOG_LIMIT=5 AGENTLOG_TEST_ARGFILE="$argfile" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--limit 5"; then
            pass "respects AGENTLOG_LIMIT environment variable"
        else
            fail "respects AGENTLOG_LIMIT environment variable" "args were: $args"
        fi
    else
        fail "respects AGENTLOG_LIMIT environment variable" "agentlog was not called"
    fi
}

# -- Run all tests --

printf "=== session-start.sh tests ===\n\n"

run_test test_exits_zero_when_agentlog_not_installed
run_test test_exits_zero_when_daemon_not_running
run_test test_identifies_git_staged_files
run_test test_identifies_git_modified_files
run_test test_deduplicates_files
run_test test_builds_correct_context_command_with_files
run_test test_no_output_when_no_relevant_decisions
run_test test_works_when_not_in_git_repo
run_test test_outputs_context_when_decisions_found
run_test test_respects_agentlog_limit_env_var

printf "\n=== Results: %d tests, %d passed, %d failed ===\n" "$TESTS_RUN" "$TESTS_PASSED" "$TESTS_FAILED"

if [[ $TESTS_FAILED -gt 0 ]]; then
    exit 1
fi
