#!/usr/bin/env bash
#
# decision-write_test.sh - Tests for the Claude Code decision-write hook.
#
# Run: bash integrations/claude-code/decision-write_test.sh
#
# Each test function sets up an isolated environment (temp dirs, mock PATHs,
# temp git repos) and asserts expected behavior. Tests are self-contained
# and clean up after themselves.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOK_SCRIPT="${SCRIPT_DIR}/decision-write.sh"

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

make_tmpdir() {
    local d
    d="$(mktemp -d)"
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

# Create a minimal git repo with an initial commit
init_git_repo() {
    local dir="$1"
    git -C "$dir" init -q
    git -C "$dir" config user.email "test@test.com"
    git -C "$dir" config user.name "Test"
    echo "initial" > "$dir/base.txt"
    git -C "$dir" add base.txt
    git -C "$dir" commit -q -m "initial"
}

# Create a mock agentlog that captures its arguments
make_mock_agentlog() {
    local mockdir="$1"
    local argfile="${2:-}"
    if [[ -n "$argfile" ]]; then
        cat > "$mockdir/agentlog" <<MOCK
#!/usr/bin/env bash
echo "\$@" >> "$argfile"
# Simulate session creation on first write (print session ID to stderr)
if ! echo "\$@" | grep -q -- "--session"; then
    echo "mock-session-id-123" >&2
fi
echo "mock-entry-id-456"
MOCK
    else
        cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
if ! echo "$@" | grep -q -- "--session"; then
    echo "mock-session-id-123" >&2
fi
echo "mock-entry-id-456"
MOCK
    fi
    chmod +x "$mockdir/agentlog"
}

# -- Tests --

test_exits_silently_when_agentlog_not_installed() {
    local tmpdir
    tmpdir="$(make_tmpdir)"

    local output exit_code
    output=$(PATH="$tmpdir" CLAUDE_SESSION_ID="test-$$" bash "$HOOK_SCRIPT" 2>&1) && exit_code=$? || exit_code=$?

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

test_exits_silently_without_session_id() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"
    make_mock_agentlog "$mockdir"

    echo "change" > "$tmpdir/new.txt"

    local output exit_code
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1) && exit_code=$? || exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 without CLAUDE_SESSION_ID"
    else
        fail "exits 0 without CLAUDE_SESSION_ID" "got exit code $exit_code"
    fi

    if [[ -z "$output" ]]; then
        pass "produces no output without CLAUDE_SESSION_ID"
    else
        fail "produces no output without CLAUDE_SESSION_ID" "got output: $output"
    fi
}

test_exits_silently_when_not_git_repo() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    make_mock_agentlog "$mockdir"

    local output exit_code
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="test-$$" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1) && exit_code=$? || exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 when not in a git repo"
    else
        fail "exits 0 when not in a git repo" "got exit code $exit_code"
    fi

    if [[ -z "$output" ]]; then
        pass "produces no output when not in a git repo"
    else
        fail "produces no output when not in a git repo" "got output: $output"
    fi
}

test_exits_silently_when_no_changes() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    # Clean repo - no changes
    local output exit_code
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="test-no-changes-$$" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1) && exit_code=$? || exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        pass "exits 0 when no changes"
    else
        fail "exits 0 when no changes" "got exit code $exit_code"
    fi

    if [[ ! -f "$argfile" ]]; then
        pass "does not call agentlog write when no changes"
    else
        fail "does not call agentlog write when no changes" "agentlog was called with: $(cat "$argfile")"
    fi

    # Clean up snapshot
    rm -rf "/tmp/agentlog-decisions/test-no-changes-$$"*
}

test_detects_unstaged_changes() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "modified" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-unstaged-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--files base.txt"; then
            pass "detects unstaged modified files"
        else
            fail "detects unstaged modified files" "args were: $args"
        fi
    else
        fail "detects unstaged modified files" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_detects_staged_changes() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "new file" > "$tmpdir/staged.txt"
    git -C "$tmpdir" add staged.txt

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-staged-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--files staged.txt"; then
            pass "detects staged files"
        else
            fail "detects staged files" "args were: $args"
        fi
    else
        fail "detects staged files" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_detects_untracked_files() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "untracked content" > "$tmpdir/newfile.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-untracked-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--files newfile.txt"; then
            pass "detects untracked files"
        else
            fail "detects untracked files" "args were: $args"
        fi
    else
        fail "detects untracked files" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_deduplicates_files() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    # Modify and stage the same file (appears in both diff --cached and diff)
    echo "v2" > "$tmpdir/base.txt"
    git -C "$tmpdir" add base.txt
    echo "v3" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-dedup-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        # base.txt should appear only once in the --files value
        local count
        count=$(echo "$args" | grep -o "base.txt" | wc -l | tr -d ' ')
        if [[ "$count" -eq 1 ]]; then
            pass "deduplicates files appearing in multiple git sources"
        else
            fail "deduplicates files appearing in multiple git sources" "found $count occurrences in: $args"
        fi
    else
        fail "deduplicates files appearing in multiple git sources" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_writes_decision_with_correct_type_and_tags() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-type-tags-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--type decision"; then
            pass "uses --type decision"
        else
            fail "uses --type decision" "args were: $args"
        fi
        if echo "$args" | grep -q -- "--tags claude-code"; then
            pass "uses default --tags claude-code"
        else
            fail "uses default --tags claude-code" "args were: $args"
        fi
    else
        fail "uses --type decision" "agentlog was not called"
        fail "uses default --tags claude-code" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_respects_agentlog_tags_env_var() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-custom-tags-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" AGENTLOG_TAGS="custom,tags" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--tags custom,tags"; then
            pass "respects AGENTLOG_TAGS environment variable"
        else
            fail "respects AGENTLOG_TAGS environment variable" "args were: $args"
        fi
    else
        fail "respects AGENTLOG_TAGS environment variable" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_title_format_few_files() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "a" > "$tmpdir/foo.go"
    echo "b" > "$tmpdir/bar.go"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-title-few-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "--title Modified"; then
            pass "title starts with 'Modified' for few files"
        else
            fail "title starts with 'Modified' for few files" "args were: $args"
        fi
    else
        fail "title starts with 'Modified' for few files" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_title_format_many_files() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    # Create 5 new files (> 3 threshold)
    echo "a" > "$tmpdir/f1.go"
    echo "b" > "$tmpdir/f2.go"
    echo "c" > "$tmpdir/f3.go"
    echo "d" > "$tmpdir/f4.go"
    echo "e" > "$tmpdir/f5.go"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-title-many-$$"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "and 4 other files"; then
            pass "title uses 'and N other files' format for many files"
        else
            fail "title uses 'and N other files' format for many files" "args were: $args"
        fi
    else
        fail "title uses 'and N other files' format for many files" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_skips_already_seen_files() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-skip-seen-$$"

    # First run - should detect base.txt as new
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        pass "first run logs the change"
    else
        fail "first run logs the change" "agentlog was not called"
    fi

    # Clear the argfile and run again with same changes
    rm -f "$argfile"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ ! -f "$argfile" ]]; then
        pass "second run skips already-seen files"
    else
        fail "second run skips already-seen files" "agentlog was called with: $(cat "$argfile")"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_detects_new_files_on_second_turn() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-second-turn-$$"

    # First run - logs base.txt
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    # Add a new file and run again
    echo "new" > "$tmpdir/extra.txt"
    rm -f "$argfile"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        if echo "$args" | grep -q -- "extra.txt"; then
            pass "detects new files on second turn"
        else
            fail "detects new files on second turn" "args were: $args"
        fi
        # Should NOT include base.txt (already seen)
        if ! echo "$args" | grep -q "base.txt"; then
            pass "excludes previously seen files on second turn"
        else
            fail "excludes previously seen files on second turn" "args were: $args"
        fi
    else
        fail "detects new files on second turn" "agentlog was not called"
        fail "excludes previously seen files on second turn" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_reuses_agentlog_session_across_turns() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-reuse-session-$$"

    # First turn - creates a new file
    echo "a" > "$tmpdir/file1.txt"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    # Second turn - creates another file
    echo "b" > "$tmpdir/file2.txt"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        local args
        args="$(cat "$argfile")"
        # The second call should include --session with the mock session ID
        local second_call
        second_call=$(tail -1 "$argfile")
        if echo "$second_call" | grep -q -- "--session mock-session-id-123"; then
            pass "reuses agentlog session ID across turns"
        else
            fail "reuses agentlog session ID across turns" "second call was: $second_call"
        fi
    else
        fail "reuses agentlog session ID across turns" "agentlog was not called"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_exits_silently_when_daemon_not_running() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    # Create a mock agentlog that fails (daemon not running)
    cat > "$mockdir/agentlog" <<'MOCK'
#!/usr/bin/env bash
echo "daemon is not running" >&2
exit 1
MOCK
    chmod +x "$mockdir/agentlog"

    local session_id="test-daemon-down-$$"
    local output exit_code
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1) && exit_code=$? || exit_code=$?

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

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_produces_no_stdout() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"
    make_mock_agentlog "$mockdir"

    local session_id="test-no-stdout-$$"
    local output
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>/dev/null)

    if [[ -z "$output" ]]; then
        pass "produces no stdout on success"
    else
        fail "produces no stdout on success" "got stdout: $output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_verbose_prints_diagnostics_to_stderr() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"
    make_mock_agentlog "$mockdir"

    local session_id="test-verbose-$$"
    local stderr_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

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

    if echo "$stderr_output" | grep -q 'command:.*agentlog write'; then
        pass "verbose mode shows command being executed"
    else
        fail "verbose mode shows command being executed" "stderr was: $stderr_output"
    fi

    if echo "$stderr_output" | grep -q 'decision written'; then
        pass "verbose mode shows write confirmation"
    else
        fail "verbose mode shows write confirmation" "stderr was: $stderr_output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_verbose_no_output_without_flag() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"
    make_mock_agentlog "$mockdir"

    local session_id="test-no-verbose-$$"
    local stderr_output
    stderr_output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if [[ -z "$stderr_output" ]]; then
        pass "no verbose output when AGENTLOG_VERBOSE is not set"
    else
        fail "no verbose output when AGENTLOG_VERBOSE is not set" "stderr was: $stderr_output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_dry_run_skips_agentlog_write() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-dry-run-$$"
    local stderr_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_DRY_RUN=1 CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if [[ ! -f "$argfile" ]]; then
        pass "dry-run does not call agentlog write"
    else
        fail "dry-run does not call agentlog write" "agentlog was called with: $(cat "$argfile")"
    fi

    if echo "$stderr_output" | grep -q '\[agentlog\] dry-run: would execute:.*agentlog write'; then
        pass "dry-run prints what would be written"
    else
        fail "dry-run prints what would be written" "stderr was: $stderr_output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_dry_run_produces_no_stdout() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"
    make_mock_agentlog "$mockdir"

    local session_id="test-dry-run-stdout-$$"
    local stdout_output
    stdout_output=$(cd "$tmpdir" && AGENTLOG_DRY_RUN=1 CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>/dev/null)

    if [[ -z "$stdout_output" ]]; then
        pass "dry-run produces no stdout"
    else
        fail "dry-run produces no stdout" "got stdout: $stdout_output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_verbose_and_dry_run_combined() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-both-flags-$$"
    local stderr_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 AGENTLOG_DRY_RUN=1 CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if [[ ! -f "$argfile" ]]; then
        pass "combined flags: does not call agentlog write"
    else
        fail "combined flags: does not call agentlog write" "agentlog was called with: $(cat "$argfile")"
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

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_telemetry_increments_counter_per_write() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-telemetry-count-$$"

    # First turn - creates a new file
    echo "a" > "$tmpdir/file1.txt"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    local count_file="/tmp/agentlog-decisions/${session_id}.count"
    if [[ -f "$count_file" ]]; then
        local count
        count=$(cat "$count_file")
        if [[ "$count" -eq 1 ]]; then
            pass "counter is 1 after first write"
        else
            fail "counter is 1 after first write" "count was: $count"
        fi
    else
        fail "counter is 1 after first write" "count file does not exist"
    fi

    # Second turn - creates another file
    echo "b" > "$tmpdir/file2.txt"
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$count_file" ]]; then
        local count
        count=$(cat "$count_file")
        if [[ "$count" -eq 2 ]]; then
            pass "counter is 2 after second write"
        else
            fail "counter is 2 after second write" "count was: $count"
        fi
    else
        fail "counter is 2 after second write" "count file does not exist"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_telemetry_verbose_shows_session_summary() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    make_mock_agentlog "$mockdir"

    local session_id="test-telemetry-verbose-$$"

    echo "change" > "$tmpdir/base.txt"
    local stderr_output
    stderr_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1 1>/dev/null)

    if echo "$stderr_output" | grep -q 'Session summary: 1 decisions captured'; then
        pass "verbose mode shows session summary with count"
    else
        fail "verbose mode shows session summary with count" "stderr was: $stderr_output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_telemetry_output_goes_to_stderr_not_stdout() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    make_mock_agentlog "$mockdir"

    local session_id="test-telemetry-stderr-$$"

    echo "change" > "$tmpdir/base.txt"
    local stdout_output
    stdout_output=$(cd "$tmpdir" && AGENTLOG_VERBOSE=1 CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>/dev/null)

    if [[ -z "$stdout_output" ]]; then
        pass "telemetry output does not leak to stdout"
    else
        fail "telemetry output does not leak to stdout" "got stdout: $stdout_output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

test_normal_operation_unchanged() {
    local tmpdir mockdir
    tmpdir="$(make_tmpdir)"
    mockdir="$(make_tmpdir)"
    init_git_repo "$tmpdir"

    echo "change" > "$tmpdir/base.txt"

    local argfile="$tmpdir/captured_args.txt"
    make_mock_agentlog "$mockdir" "$argfile"

    local session_id="test-normal-op-$$"
    local output
    output=$(cd "$tmpdir" && CLAUDE_SESSION_ID="$session_id" PATH="$mockdir:/usr/bin:/bin" bash "$HOOK_SCRIPT" 2>&1)

    if [[ -f "$argfile" ]]; then
        pass "normal operation: agentlog write is called"
    else
        fail "normal operation: agentlog write is called" "agentlog was not called"
    fi

    if [[ -z "$output" ]]; then
        pass "normal operation: no output produced"
    else
        fail "normal operation: no output produced" "got output: $output"
    fi

    rm -rf "/tmp/agentlog-decisions/${session_id}"*
}

# -- Run all tests --

printf "=== decision-write.sh tests ===\n\n"

run_test test_exits_silently_when_agentlog_not_installed
run_test test_exits_silently_without_session_id
run_test test_exits_silently_when_not_git_repo
run_test test_exits_silently_when_no_changes
run_test test_detects_unstaged_changes
run_test test_detects_staged_changes
run_test test_detects_untracked_files
run_test test_deduplicates_files
run_test test_writes_decision_with_correct_type_and_tags
run_test test_respects_agentlog_tags_env_var
run_test test_title_format_few_files
run_test test_title_format_many_files
run_test test_skips_already_seen_files
run_test test_detects_new_files_on_second_turn
run_test test_reuses_agentlog_session_across_turns
run_test test_exits_silently_when_daemon_not_running
run_test test_produces_no_stdout
run_test test_verbose_prints_diagnostics_to_stderr
run_test test_verbose_no_output_without_flag
run_test test_dry_run_skips_agentlog_write
run_test test_dry_run_produces_no_stdout
run_test test_verbose_and_dry_run_combined
run_test test_telemetry_increments_counter_per_write
run_test test_telemetry_verbose_shows_session_summary
run_test test_telemetry_output_goes_to_stderr_not_stdout
run_test test_normal_operation_unchanged

printf "\n=== Results: %d tests, %d passed, %d failed ===\n" "$TESTS_RUN" "$TESTS_PASSED" "$TESTS_FAILED"

if [[ $TESTS_FAILED -gt 0 ]]; then
    exit 1
fi
