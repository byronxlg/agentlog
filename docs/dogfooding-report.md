# Dogfooding Report: Using agentlog to build agentlog

Phase 8 of the agentlog project introduced dogfooding as a core validation strategy: install agentlog's own auto-ingestion hooks in the agentlog repo, then use the tool to capture decisions while building the remaining Phase 8 features (export command, SDK methods, launch prep).

This report documents what happened.

## Setup

Two Claude Code hooks were installed via PR #296 (issue #281):

**Session-start hook** (`session-start.sh`) - runs on `UserPromptSubmit`:
- Queries the agentlog daemon for decisions relevant to the current working set of files
- Collects files from git state (staged, unstaged modified, last commit)
- Falls back to repo/directory name as a topic if no files are found
- Runs once per session using a marker file at `/tmp/agentlog-session-{CLAUDE_SESSION_ID}`
- Exits silently when agentlog is not installed, the daemon is not running, or no results are found

**Decision-write hook** (`decision-write.sh`) - runs on `Stop`:
- Detects files modified during the current turn by comparing git state against a per-session snapshot stored at `/tmp/agentlog-decisions/{session_id}.files`
- Only logs new file changes not seen in previous turns
- Builds a descriptive title (e.g., "Modified foo.go bar.go" or "Modified foo.go and 4 other files")
- Tracks agentlog session ID across turns for grouping related decisions
- Tags decisions with "claude-code" by default (configurable via `AGENTLOG_TAGS`)

Both hooks are designed to be invisible when things are working - no output on success, silent exit on failure. This is intentional: hooks that produce noise get disabled.

## What happened

After hooks were installed (PR #296 merged 2026-03-17), the following development work happened with hooks active:

- **Issue #283**: Python SDK `export()` method (PR #323, merged 2026-03-18)
- **Issue #284**: TypeScript SDK `export()` method (PR #326, opened 2026-03-18, in review)

This gave us two full build-review cycles with the hooks running. Earlier Phase 8 work (issues #281 and #282) happened before the hooks were installed and does not count as dogfooding.

## What worked

### Silent operation

The hooks ran without any agent reporting errors, crashes, or interference. Across 18+ status updates from Builder, Reviewer, Lead, and Director roles, not a single mention of hook-related problems. The friction tracker (issue #295) received zero entries.

This is the most important finding: **the hooks did not get in the way**. For auto-instrumentation that runs on every prompt submission and every response, "invisible" is exactly the right behavior. Hooks that interrupt workflow get disabled immediately.

### Graceful degradation

The hooks handle edge cases without failing:
- No agentlog installed: silent exit
- Daemon not running: silent exit
- No relevant decisions found: silent exit
- No git repo: silent exit
- No `CLAUDE_SESSION_ID`: silent exit (decision-write only)

This means the hooks can be committed to a shared repo without breaking anything for developers who do not use agentlog. The install script adds them to `.claude/settings.json`, but if the binary is not present, they are no-ops.

### Per-session deduplication

The decision-write hook tracks which files it has already logged per session, avoiding duplicate entries when the same files are modified across multiple turns. The session-start hook runs only once per session via a marker file. Both mechanisms worked correctly - no duplicate decisions or repeated context injection were reported.

## What did not work

### Limited observability into what was captured

No agent explicitly commented on what context was injected by the session-start hook or what decisions were captured by the decision-write hook. The hooks are silent by design, but this creates a visibility gap: we know the hooks ran, but we do not know how useful the injected context was to agents during their sessions.

There is no way for an agent (or a human) to confirm mid-session what agentlog injected without querying the daemon separately. This is a deliberate design choice (avoid noise), but it means the dogfooding data about context quality is absent.

### Single-account constraint limited review visibility

All agent roles (Builder, Reviewer, Lead, Director) share the `byronxlg` GitHub account. This meant the Reviewer could not submit formal GitHub review approvals - only comments. This is a process limitation, not an agentlog limitation, but it affected the team's ability to dogfood the review workflow cleanly. Engineering discussion #272 documents this constraint and the workarounds adopted.

### Empty friction tracker

The friction tracker (issue #295) received zero entries across all of Phase 8. The Lead noted this repeatedly in status updates. The Director identified three possible interpretations:

1. The hooks work well and there is nothing to report
2. Not enough development cycles happened with hooks active
3. Agents are not conditioned to report friction

Interpretation 2 is the most likely. The hooks were only active for two build cycles (Python and TypeScript SDK export methods). The first build cycle on #283 was also delayed by a builder stall (issue sat In Progress for 24+ hours with no activity before being reassigned).

### No cross-session context validation

The session-start hook queries for past decisions to inject into new sessions. But since all Phase 8 work happened in a compressed timeframe on separate issues, there was limited opportunity for one session's captured decisions to be useful in a subsequent session. The real value of context injection would show in longer-running projects where developers return to the same files days or weeks later.

## Changes made as a result of dogfooding

No changes were made to the hooks, daemon, SDKs, or CLI based on dogfooding. The friction tracker was empty and no bugs were discovered through real usage.

However, several process changes were made during Phase 8 that are relevant:

- **PR review enforcement** (Engineering discussion #272, Director discussion #277): The Director mandated that all Phase 8 PRs require Reviewer approval before merge, addressing a gap where seven phases of code shipped without formal review.
- **Hook sync CI check** (PR #296): A CI step was added to verify that the installed hooks in `.claude/hooks/` stay in sync with the source scripts in `integrations/claude-code/`. This prevents drift between the canonical hook scripts and their installed copies.

## Recommendations

### Short-term (next phase)

1. **Add a `--verbose` or `--dry-run` flag to hooks**: Let developers see what would be captured or injected without modifying behavior. This would help validate that hooks are capturing the right information and injecting useful context.

2. **Extend dogfooding duration**: Two build cycles is not enough data. Keep the hooks active through Phase 9 and beyond. The real value of context injection appears in longer-running projects.

3. **Add hook telemetry**: Track basic metrics (number of decisions captured per session, number of context entries injected, query latency) to a local file or the daemon itself. This data would make future dogfooding reports more concrete.

### Medium-term

4. **Test with multiple repos**: The current dogfooding is single-repo. Validate the hooks in a multi-repo setup where decisions from one project might be relevant to another.

5. **Validate context usefulness**: Design a structured way to measure whether injected context actually influences agent behavior. This could be as simple as a post-session survey prompt or as involved as A/B testing sessions with and without context injection.

6. **Improve friction reporting**: If agents are not naturally inclined to report friction, consider adding a periodic prompt or hook that asks "did anything about agentlog get in your way this session?" Zero friction entries might indicate a feedback mechanism gap rather than a frictionless experience.

## Conclusion

The dogfooding setup worked as infrastructure: hooks installed cleanly, ran without errors, and did not interfere with development. The "silent when working" design philosophy proved correct for auto-instrumentation.

The limitation is that "working silently" makes it hard to evaluate whether the tool is providing value. We know agentlog did not break anything. We do not yet have strong evidence that it helped. This is the right question for Phase 9: move from "does it work?" to "does it help?"
