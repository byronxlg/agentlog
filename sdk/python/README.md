# agentlog Python SDK

Python client for [agentlog](https://github.com/byronxlg/agentlog) - a local-first decision log daemon for agentic workflows.

## Installation

```bash
pip install agentlog-sdk
```

## Quickstart

```python
import agentlog

agentlog.write("decision", "Use PostgreSQL for persistence")
entries = agentlog.query("database")
```

## Requirements

- Python 3.9+
- A running `agentlogd` daemon (see the main project README)

## Usage

### Writing entries

```python
import agentlog

# Write a decision entry (session created automatically)
agentlog.write(
    "decision",
    "Use Redis for caching",
    body="Redis provides sub-millisecond reads and built-in TTL support.",
    tags=["infrastructure", "caching"],
    files=["config/redis.yaml"],
)

# Supported entry types: decision, attempt_failed, deferred, assumption, question
agentlog.write("assumption", "All users have Python 3.9+")
agentlog.write("question", "Should we use async or sync HTTP client?")
```

### Searching entries

```python
# Full-text search
results = agentlog.query("database migration")

# Search with filters
results = agentlog.query("caching", type="decision", limit=5)
```

### Listing entries

```python
# List entries by type
entries = agentlog.log(type="decision")

# List entries by session
entries = agentlog.log(session="your-session-id")

# List entries by tag
entries = agentlog.log(tag="infrastructure")

# List entries from the last hour
entries = agentlog.log(since="1h")
```

### Getting context for prompts

```python
# Get a formatted text block for prompt injection
context = agentlog.context(query="authentication")
print(context)
# Output:
# # Recent decisions
#
# ## [decision] Use JWT for API auth (2026-03-15 10:30)
# JWTs are stateless and work well with our microservices architecture.
# Tags: auth, api
# Files: internal/auth/jwt.go
```

### Using the client class directly

```python
from agentlog import AgentlogClient

# Custom socket path
client = AgentlogClient(agentlog_dir="/custom/path")

# Or explicit socket path
client = AgentlogClient(socket_path="/tmp/agentlogd.sock")

# All methods are available on the client instance
entry_id = client.write("decision", "Use gRPC for internal services")
```

### Configuration

The SDK looks for the daemon socket at `~/.agentlog/agentlogd.sock` by default.
Override this with:

- The `AGENTLOG_DIR` environment variable
- The `agentlog_dir` constructor argument
- The `socket_path` constructor argument (takes precedence)

### Error handling

```python
from agentlog import AgentlogError, ConnectionError, DaemonNotRunning

try:
    agentlog.write("decision", "Test entry")
except DaemonNotRunning:
    print("Start the daemon first: agentlog start")
except ConnectionError as e:
    print(f"Connection failed: {e}")
except AgentlogError as e:
    print(f"Unexpected error: {e}")
```

## Development

```bash
# Install in development mode
pip install -e sdk/python/

# Run tests
python -m pytest sdk/python/tests/ -v

# Run only unit tests (no daemon required)
python -m pytest sdk/python/tests/test_client.py -v
```
