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

test_agentlog_topic_override_in_git_repo() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    # Create a git repo - normally the repo name would be used as fallback topic
    git -C "$tmpdir" init -q
    git -C "$tmpdir" config user.email "test@test.com"
    git -C "$tmpdir" config user.name "Test"
    echo "a" > "$tmpdir/file.txt"
    git -C "$tmpdir" add file.txt
    git -C "$tmpdir" commit -q -m "initial"

    # No modified/staged files, so the script falls back to --topic.
    # With AGENTLOG_TOPIC set, it should use that instead of the repo name.

    local mockdir
    mockdir="$(make_tmpdir)"
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "$@" > "${AGENTLOG_TEST_ARGFILE}"
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local argfile="$tmpdir/captured_args.txt"
    output=$(cd "$tmpdir" && AGENTLOG_TOPIC="custom-topic" AGENTLOG_TEST_ARGFILE="$argfile" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--topic custom-topic"; then
            pass "AGENTLOG_TOPIC overrides repo name in git repo"
        else
            fail "AGENTLOG_TOPIC overrides repo name in git repo" "args were: $args"
        fi
    else
        fail "AGENTLOG_TOPIC overrides repo name in git repo" "agentlog was not called"
    fi
}

test_skips_query_on_second_run_in_same_session() {
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
    local call_count_file="$tmpdir/call_count"
    echo "0" > "$call_count_file"
    cat > "$mockdir/agentlog" <<MOCK
#!/usr/bin/env bash
count=\$(cat "$call_count_file")
echo \$((count + 1)) > "$call_count_file"
echo "# Relevant decisions"
MOCK
    chmod +x "$mockdir/agentlog"

    local session_id="test-session-$$"
    # Clean up any stale marker from a prior run
    rm -f "/tmp/agentlog-session-${session_id}"

    # First invocation - should call agentlog and produce output
    output1=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    # Second invocation - should exit early due to marker file
    output2=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    local count
    count="$(cat "$call_count_file")"

    if [[ "$count" -eq 1 ]]; then
        pass "agentlog called only once across two invocations with same session ID"
    else
        fail "agentlog called only once across two invocations with same session ID" "called $count times"
    fi

    if [[ -n "$output1" ]]; then
        pass "first invocation produces output"
    else
        fail "first invocation produces output" "output was empty"
    fi

    if [[ -z "$output2" ]]; then
        pass "second invocation produces no output (skipped)"
    else
        fail "second invocation produces no output (skipped)" "got output: $output2"
    fi

    # Clean up marker
    rm -f "/tmp/agentlog-session-${session_id}"
}

test_runs_every_time_without_session_id() {
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
    local call_count_file="$tmpdir/call_count"
    echo "0" > "$call_count_file"
    cat > "$mockdir/agentlog" <<MOCK
#!/usr/bin/env bash
count=\$(cat "$call_count_file")
echo \$((count + 1)) > "$call_count_file"
echo "# Relevant decisions"
MOCK
    chmod +x "$mockdir/agentlog"

    # Two invocations without CLAUDE_SESSION_ID - both should call agentlog
    output1=$(cd "$tmpdir" && PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)
    output2=$(cd "$tmpdir" && PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    local count
    count="$(cat "$call_count_file")"

    if [[ "$count" -eq 2 ]]; then
        pass "agentlog called on every invocation when CLAUDE_SESSION_ID is not set"
    else
        fail "agentlog called on every invocation when CLAUDE_SESSION_ID is not set" "called $count times"
    fi
}

test_verbose_prints_diagnostics_to_stderr() {
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
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local stderr_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if echo "$stderr_output" | grep -q '\[agentlog\]'; then
        pass "verbose mode prints diagnostic output to stderr"
    else
        fail "verbose mode prints diagnostic output to stderr" "stderr was: $stderr_output"
    fi

    if echo "$stderr_output" | grep -q 'detected.*file'; then
        pass "verbose mode shows files detected"
    else
        fail "verbose mode shows files detected" "stderr was: $stderr_output"
    fi

    if echo "$stderr_output" | grep -q 'command:.*agentlog context'; then
        pass "verbose mode shows command being executed"
    else
        fail "verbose mode shows command being executed" "stderr was: $stderr_output"
    fi
}

test_verbose_no_output_without_flag() {
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
echo "No relevant decisions found."
MOCK
    chmod +x "$mockdir/agentlog"

    local stderr_output
    stderr_output=$(cd "$tmpdir" && PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if [[ -z "$stderr_output" ]]; then
        pass "no verbose output when AGENTLOG_VERBOSE is not set"
    else
        fail "no verbose output when AGENTLOG_VERBOSE is not set" "stderr was: $stderr_output"
    fi
}

test_dry_run_skips_agentlog_context() {
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
    local argfile="$tmpdir/captured_args.txt"
    cat > "$mockdir/agentlog" <<MOCK
#!/usr/bin/env bash
echo "\$@" > "$argfile"
echo "# Relevant decisions"
MOCK
    chmod +x "$mockdir/agentlog"

    local stderr_output stdout_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_DRY_RUN=1 PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)
    stdout_output=$(cd "$tmpdir" && AGENTLOG_DRY_RUN=1 PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>/dev/null)

    if [[ ! -f "$argfile" ]]; then
        pass "dry-run does not call agentlog context"
    else
        fail "dry-run does not call agentlog context" "agentlog was called with: $(cat "$argfile")"
    fi

    if echo "$stderr_output" | grep -q '\[agentlog\] dry-run: would execute:.*agentlog context'; then
        pass "dry-run prints what would be queried"
    else
        fail "dry-run prints what would be queried" "stderr was: $stderr_output"
    fi

    if [[ -z "$stdout_output" ]]; then
        pass "dry-run produces no stdout"
    else
        fail "dry-run produces no stdout" "got stdout: $stdout_output"
    fi
}

test_verbose_and_dry_run_combined() {
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
    local argfile="$tmpdir/captured_args.txt"
    cat > "$mockdir/agentlog" <<MOCK
#!/usr/bin/env bash
echo "\$@" > "$argfile"
echo "# Relevant decisions"
MOCK
    chmod +x "$mockdir/agentlog"

    local stderr_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 AGENTLOG_DRY_RUN=1 PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if [[ ! -f "$argfile" ]]; then
        pass "combined flags: does not call agentlog context"
    else
        fail "combined flags: does not call agentlog context" "agentlog was called with: $(cat "$argfile")"
    fi

    if echo "$stderr_output" | grep -q '\[agentlog\] dry-run: would execute:'; then
        pass "combined flags: prints dry-run message"
    else
        fail "combined flags: prints dry-run message" "stderr was: $stderr_output"
    fi

    if echo "$stderr_output" | grep -q 'detected.*file'; then
        pass "combined flags: prints verbose diagnostics"
    else
        fail "combined flags: prints verbose diagnostics" "stderr was: $stderr_output"
    fi
}

test_telemetry_counts_context_entries() {
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
cat <<'CONTEXT'
# Relevant decisions

## [decision] Use structured logging (2025-01-15T10:30:00Z)
Switched from fmt.Println to slog for all log output.
Files: internal/daemon/server.go
Tags: architecture

## [decision] Use SQLite for index (2025-01-15T11:00:00Z)
Chose SQLite over PostgreSQL for the local index.
Files: internal/index/store.go
Tags: architecture

## [assumption] Single user per daemon (2025-01-15T11:30:00Z)
Assuming one user per daemon instance.
Tags: design
CONTEXT
MOCK
    chmod +x "$mockdir/agentlog"

    local session_id="test-telemetry-count-$$"
    rm -f "/tmp/agentlog-session-${session_id}"

    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>/dev/null)

    local marker="/tmp/agentlog-session-${session_id}"
    if [[ -f "$marker" ]]; then
        local count
        count=$(cat "$marker")
        if [[ "$count" -eq 3 ]]; then
            pass "marker file stores context entry count (3)"
        else
            fail "marker file stores context entry count (3)" "count was: $count"
        fi
    else
        fail "marker file stores context entry count (3)" "marker file does not exist"
    fi

    rm -f "/tmp/agentlog-session-${session_id}"
}

test_telemetry_verbose_shows_session_summary() {
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
cat <<'CONTEXT'
# Relevant decisions

## [decision] Use structured logging (2025-01-15T10:30:00Z)
Switched from fmt.Println to slog for all log output.
Files: internal/daemon/server.go
Tags: architecture
CONTEXT
MOCK
    chmod +x "$mockdir/agentlog"

    local stderr_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if echo "$stderr_output" | grep -q 'Session summary: 1 context entries injected'; then
        pass "verbose mode shows session summary with entry count"
    else
        fail "verbose mode shows session summary with entry count" "stderr was: $stderr_output"
    fi
}

test_telemetry_output_goes_to_stderr_not_stdout() {
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
cat <<'CONTEXT'
# Relevant decisions

## [decision] Test decision (2025-01-15T10:30:00Z)
Test body
CONTEXT
MOCK
    chmod +x "$mockdir/agentlog"

    local stdout_output
    stdout_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>/dev/null)

    # Stdout should only contain the context output, not telemetry lines
    if echo "$stdout_output" | grep -q 'Session summary'; then
        fail "telemetry summary does not leak to stdout" "got stdout containing: Session summary"
    else
        pass "telemetry summary does not leak to stdout"
    fi
}

test_normal_operation_unchanged() {
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
cat <<'CONTEXT'
# Relevant decisions

## [decision] Test decision (2025-01-15T10:30:00Z)
Test body
CONTEXT
MOCK
    chmod +x "$mockdir/agentlog"

    local output
    output=$(cd "$tmpdir" && PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>/dev/null)

    if echo "$output" | grep -q "# Relevant decisions"; then
        pass "normal operation: context is output to stdout"
    else
        fail "normal operation: context is output to stdout" "got output: $output"
    fi

    local stderr_output
    stderr_output=$(cd "$tmpdir" && PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if [[ -z "$stderr_output" ]]; then
        pass "normal operation: no stderr output"
    else
        fail "normal operation: no stderr output" "got stderr: $stderr_output"
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
run_test test_agentlog_topic_override_in_git_repo
run_test test_skips_query_on_second_run_in_same_session
run_test test_runs_every_time_without_session_id
run_test test_verbose_prints_diagnostics_to_stderr
run_test test_verbose_no_output_without_flag
run_test test_dry_run_skips_agentlog_context
run_test test_verbose_and_dry_run_combined
run_test test_telemetry_counts_context_entries
run_test test_telemetry_verbose_shows_session_summary
run_test test_telemetry_output_goes_to_stderr_not_stdout
run_test test_normal_operation_unchanged

printf "\n=== Results: %d tests, %d passed, %d failed ===\n" "$TESTS_RUN" "$TESTS_PASSED" "$TESTS_FAILED"

if [[ $TESTS_FAILED -gt 0 ]]; then
    exit 1
fi
