# Show HN Draft

**Title:** Show HN: agentlog - Decision log daemon for AI agent workflows

**URL:** https://github.com/byronxlg/agentlog

**Text:**

agentlog is a local-first decision log daemon that captures why AI agents make decisions, not just what they change.

The problem: when AI agents work on your codebase, critical context disappears. Git tells you what changed. LLM traces capture the conversation. Neither tells you why a particular approach was chosen, what alternatives were considered, or what failed before the final solution.

agentlog fills that gap with structured decision entries (decisions, failed attempts, assumptions, deferred work, open questions) stored as append-only JSONL files with a SQLite index for fast queries.

Key details:
- Pure Go daemon, single binary, no external dependencies
- JSONL source of truth (human-readable, git-committable) + rebuildable SQLite index
- CLI with write, query, blame, context, and export commands
- Python and TypeScript SDKs
- Claude Code integration via hooks - auto-captures decisions and injects past context into new sessions
- Export with built-in templates (PR summary, retrospective, handoff)

We dogfooded it by using agentlog to build agentlog. The Claude Code hooks ran silently across 18+ agent sessions without a single reported issue. The key design insight: auto-instrumentation that runs on every prompt and response must be invisible when working. Hooks that produce noise get disabled immediately.

The honest finding from dogfooding: we know agentlog does not break anything. We do not yet have strong evidence that it helps. The next step is measuring whether injected context actually influences agent behavior - moving from "does it work?" to "does it help?"

Source: https://github.com/byronxlg/agentlog
Docs: https://byronxlg.github.io/agentlog/docs/
