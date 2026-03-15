# agentlog CLAUDE.md snippet

Copy this section into your project's CLAUDE.md to enable decision logging.

---

## Decision logging

This project uses [agentlog](https://github.com/byronxlg/agentlog) to capture
decision history during coding sessions. Log significant decisions as you work
so that future sessions have context about what was tried, what worked, and why.

### Entry types

| Type | When to use |
|------|-------------|
| `decision` | Choosing between alternatives |
| `attempt_failed` | Something you tried that didn't work |
| `deferred` | Work you chose to skip or postpone |
| `assumption` | An assumption that could be wrong |
| `question` | An open question you can't answer from context |

### How to log

```bash
agentlog write --type <type> --title "<short description>" \
  --body "<reasoning and context>" \
  --tags "<comma-separated tags>" \
  --files "<comma-separated file paths affected>"
```

### When to log

- Before making a non-obvious choice between alternatives
- When you try an approach and it fails (log before moving on)
- When you defer work, skip a refactor, or leave a TODO
- When you assume something about the codebase, requirements, or environment
- When you have a question you can't answer from the available context

### Before modifying a file

Check for past decisions related to the file:

```bash
agentlog blame <file>
```

Use this context to avoid repeating failed approaches or contradicting prior decisions.

### Examples

Logging a design decision:

```bash
agentlog write --type decision \
  --title "Use write-ahead logging for SQLite" \
  --body "WAL mode gives concurrent readers without blocking writes. Trade-off: slightly more complex recovery, but worth it for our read-heavy query patterns." \
  --tags "database,performance" \
  --files "internal/index/index.go"
```

Logging a failed attempt:

```bash
agentlog write --type attempt_failed \
  --title "Tried using flock for cross-process locking" \
  --body "flock does not work reliably on NFS. Switching to advisory locks via the daemon instead." \
  --tags "locking,reliability" \
  --files "internal/store/lock_unix.go"
```

Logging a deferred task:

```bash
agentlog write --type deferred \
  --title "Connection pooling for daemon socket" \
  --body "Single connection per request is fine for current load. Pool when we see latency issues." \
  --tags "performance,daemon"
```
