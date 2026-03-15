# Architecture

This page describes how agentlog works internally.

## Overview

```
agentlog (CLI) --> agentlogd (daemon) --> JSONL files (source of truth)
                                      --> SQLite index (derived, rebuildable)
```

The CLI is a thin client that sends JSON messages to the daemon over a Unix socket. The daemon owns all data access - it appends entries to JSONL log files and keeps a SQLite index updated for fast queries. The JSONL files are the source of truth; the index can be rebuilt from them at any time.

## Components

### CLI (`agentlog`)

The CLI binary parses commands and flags, then sends a JSON request to the daemon's Unix socket. It does not access the log files or database directly. Each command maps to a daemon RPC method:

| Command | Daemon method |
|---------|---------------|
| `write` | `write` |
| `log` | `query` |
| `query` | `search` |
| `show` | `list_sessions` + `get_session` |
| `blame` | `blame` |

The `start` and `stop` commands manage the daemon process directly (fork/SIGTERM) without going through the socket.

### Daemon (`agentlogd`)

The daemon is a long-running background process. On startup it:

1. Creates the data directory (`~/.agentlog/`) if it does not exist
2. Checks for a stale PID file and cleans it up if the process is not running
3. Opens the SQLite index (creating it if needed, running schema migrations)
4. Opens a log file for its own structured logs (`agentlogd.log`)
5. Writes its PID to `agentlogd.pid`
6. Listens on a Unix socket (`agentlogd.sock`)
7. Starts a write serialization goroutine

The daemon handles one JSON request per connection. Each connection is handled in its own goroutine. Writes are serialized through a single channel to prevent concurrent JSONL file corruption.

On shutdown (SIGTERM), the daemon:

1. Stops accepting new connections
2. Drains pending writes from the write channel
3. Waits for in-flight request handlers to complete
4. Closes the SQLite index
5. Removes the PID file and socket file

### Protocol

The daemon uses a simple JSON-over-Unix-socket protocol. Each message is a single line of JSON terminated by a newline.

**Request format:**

```json
{
  "method": "write",
  "params": { ... }
}
```

**Response format:**

```json
{
  "ok": true,
  "result": { ... }
}
```

On error:

```json
{
  "ok": false,
  "error": "error message"
}
```

## Data storage

### JSONL log store

The primary data store is a set of JSONL (JSON Lines) files in `~/.agentlog/log/`. Each session gets its own file named `<session_id>.jsonl`. Each line in a file is a single JSON-encoded entry.

Key properties:

- **Append-only** - entries are only added, never modified or deleted
- **One file per session** - keeps related entries together and limits file size
- **File locking** - uses `flock` (Unix) to prevent corruption from concurrent writes
- **Human-readable** - files can be inspected with standard text tools (`cat`, `grep`, `jq`)
- **Git-committable** - JSONL files can be checked into version control alongside code

**Entry schema:**

```json
{
  "id": "unique-entry-id",
  "timestamp": "2026-03-15T10:30:00Z",
  "session_id": "session-uuid",
  "type": "decision",
  "title": "Use SQLite for the index",
  "body": "Reasoning and context...",
  "tags": ["architecture", "database"],
  "file_refs": ["internal/index/index.go"]
}
```

### SQLite index

The SQLite database at `~/.agentlog/index.db` is a derived cache that enables fast queries. It uses WAL mode for concurrent read access and contains:

- **`entries` table** - core entry data (id, timestamp, session_id, type, title, body)
- **`entry_tags` table** - normalized tags with foreign key to entries
- **`entry_file_refs` table** - normalized file references with foreign key to entries
- **`entries_fts` virtual table** - FTS5 full-text search index over title and body

The FTS5 index is kept in sync with the entries table via triggers (after insert, update, delete).

Since the index is derived from the JSONL files, it can be dropped and rebuilt at any time with no data loss. The `Rebuild` function reads all JSONL files, drops all tables, recreates the schema, and re-indexes every entry.

## Data flow

### Write path

1. CLI parses `--type`, `--title`, and other flags
2. CLI sends a `write` request with entry params to the daemon socket
3. Daemon assigns an entry ID, timestamp, and session ID (if not provided)
4. Daemon sends the entry to the write channel
5. Write goroutine appends the entry as a JSON line to `log/<session_id>.jsonl` (with file lock)
6. Write goroutine inserts the entry into the SQLite index (entries, tags, file_refs tables)
7. Daemon returns the entry (including generated ID) to the CLI
8. CLI prints the entry ID

### Query path

1. CLI parses filter flags and sends a `query` request to the daemon socket
2. Daemon queries the SQLite index with the provided filters
3. Daemon returns matching entries as JSON
4. CLI sorts entries (newest-first) and prints them

### Search path

1. CLI parses the search term and sends a `search` request to the daemon socket
2. Daemon runs an FTS5 query against the `entries_fts` virtual table
3. Daemon returns matching entries as JSON
4. CLI applies any additional client-side filters (type, session, tag, time range)
5. CLI formats and prints results with matching terms highlighted

## Directory layout

All data is stored under `~/.agentlog/` by default (overridable with `--dir`):

```
~/.agentlog/
  agentlogd.sock    # Unix socket for CLI-to-daemon communication
  agentlogd.pid     # PID file for the running daemon
  agentlogd.log     # Daemon structured log (JSON)
  index.db          # SQLite index (WAL mode)
  index.db-wal      # SQLite WAL file
  index.db-shm      # SQLite shared memory file
  log/
    <session-1>.jsonl   # Entries for session 1
    <session-2>.jsonl   # Entries for session 2
    ...
```
