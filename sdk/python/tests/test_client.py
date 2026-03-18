"""Unit tests for AgentlogClient.

Tests cover serialization, error handling, session management, and type
validation. The daemon socket is mocked so no running daemon is required.
"""

from __future__ import annotations

import json
import os
import shutil
import socket
import tempfile
import threading
import uuid
from pathlib import Path
from typing import Any
from unittest.mock import patch

import pytest

from agentlog.client import AgentlogClient, VALID_ENTRY_TYPES, _parse_duration, _resolve_time
from agentlog.errors import AgentlogError, ConnectionError, DaemonNotRunning


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def sock_dir():
    """Create a short temp directory under /tmp for Unix socket files.

    Unix sockets have a path length limit (~104 bytes on macOS), so we avoid
    the long paths that pytest's tmp_path produces.
    """
    d = tempfile.mkdtemp(prefix="al_", dir="/tmp")
    yield d
    shutil.rmtree(d, ignore_errors=True)


@pytest.fixture
def sock_path(sock_dir):
    """Return a short socket path safe for Unix socket binding."""
    return os.path.join(sock_dir, "t.sock")


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _make_ok_response(result: Any) -> bytes:
    """Build a daemon-style OK response line."""
    resp = {"ok": True, "result": result}
    return json.dumps(resp).encode("utf-8") + b"\n"


def _make_error_response(msg: str) -> bytes:
    """Build a daemon-style error response line."""
    resp = {"ok": False, "error": msg}
    return json.dumps(resp).encode("utf-8") + b"\n"


class FakeDaemon:
    """A minimal Unix socket server that records requests and replies with
    canned responses.

    Supports two modes:
    - Single response: set via set_response() or set_error(), reused for all requests.
    - Response sequence: set via set_responses(), each request pops the next response.

    Usage::

        with FakeDaemon(socket_path) as daemon:
            daemon.set_response({"session_id": "abc"})
            client = AgentlogClient(socket_path=socket_path)
            client._send("create_session")
            assert daemon.last_request["method"] == "create_session"
    """

    def __init__(self, socket_path: str) -> None:
        self.socket_path = socket_path
        self._response: Any = None
        self._error: str | None = None
        self._response_queue: list[Any] | None = None
        self.requests: list[dict[str, Any]] = []
        self._server_sock: socket.socket | None = None
        self._thread: threading.Thread | None = None
        self._running = False

    @property
    def last_request(self) -> dict[str, Any] | None:
        return self.requests[-1] if self.requests else None

    def set_response(self, result: Any) -> None:
        """Set the response result (ok=True) reused for all subsequent requests."""
        self._response = result
        self._error = None
        self._response_queue = None

    def set_responses(self, results: list[Any]) -> None:
        """Set a sequence of responses; each request consumes the next one."""
        self._response_queue = list(results)
        self._response = None
        self._error = None

    def set_error(self, msg: str) -> None:
        """Set the next response to be an error."""
        self._error = msg
        self._response = None
        self._response_queue = None

    def __enter__(self) -> FakeDaemon:
        self._server_sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self._server_sock.bind(self.socket_path)
        self._server_sock.listen(5)
        self._server_sock.settimeout(5.0)
        self._running = True
        self._thread = threading.Thread(target=self._serve, daemon=True)
        self._thread.start()
        return self

    def __exit__(self, *args: Any) -> None:
        self._running = False
        if self._server_sock:
            self._server_sock.close()
        if self._thread:
            self._thread.join(timeout=2.0)

    def _next_response_bytes(self) -> bytes:
        """Determine the response bytes to send for the current request."""
        if self._error:
            return _make_error_response(self._error)
        if self._response_queue:
            result = self._response_queue.pop(0)
            return _make_ok_response(result)
        return _make_ok_response(self._response)

    def _serve(self) -> None:
        while self._running:
            try:
                conn, _ = self._server_sock.accept()
            except (socket.timeout, OSError):
                continue
            try:
                data = b""
                while b"\n" not in data:
                    chunk = conn.recv(4096)
                    if not chunk:
                        break
                    data += chunk
                if data.strip():
                    req = json.loads(data)
                    self.requests.append(req)
                    conn.sendall(self._next_response_bytes())
            finally:
                conn.close()


# ---------------------------------------------------------------------------
# Type validation tests
# ---------------------------------------------------------------------------

class TestTypeValidation:
    """Tests for entry type validation."""

    def test_valid_types_accepted(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            client = AgentlogClient(socket_path=sock_path)
            client._session_id = "sess-1"
            for entry_type in sorted(VALID_ENTRY_TYPES):
                daemon.set_response({
                    "id": f"entry-{entry_type}",
                    "timestamp": "2026-03-15T10:00:00Z",
                    "session_id": "sess-1",
                    "type": entry_type,
                    "title": "Test",
                })
                entry_id = client.write(type=entry_type, title="Test")
                assert entry_id == f"entry-{entry_type}"

    def test_invalid_type_raises_value_error(self) -> None:
        client = AgentlogClient(socket_path="/nonexistent.sock")
        with pytest.raises(ValueError, match="invalid entry type"):
            client.write(type="invalid_type", title="Test")

    def test_valid_entry_types_constant(self) -> None:
        assert VALID_ENTRY_TYPES == {
            "decision", "attempt_failed", "deferred", "assumption", "question",
        }


# ---------------------------------------------------------------------------
# Serialization tests
# ---------------------------------------------------------------------------

class TestSerialization:
    """Tests for request serialization sent to the daemon."""

    def test_write_serializes_entry(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response({
                "id": "entry-1",
                "timestamp": "2026-03-15T10:00:00Z",
                "session_id": "sess-1",
                "type": "decision",
                "title": "Use Redis",
                "body": "For caching",
                "tags": ["infrastructure"],
                "file_refs": ["config.yaml"],
            })
            client = AgentlogClient(socket_path=sock_path)
            client._session_id = "sess-1"

            client.write(
                type="decision",
                title="Use Redis",
                body="For caching",
                tags=["infrastructure"],
                files=["config.yaml"],
            )

            req = daemon.last_request
            assert req["method"] == "write"
            entry = req["params"]["entry"]
            assert entry["type"] == "decision"
            assert entry["title"] == "Use Redis"
            assert entry["body"] == "For caching"
            assert entry["tags"] == ["infrastructure"]
            assert entry["file_refs"] == ["config.yaml"]
            assert entry["session_id"] == "sess-1"

    def test_write_omits_optional_fields_when_none(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response({
                "id": "entry-1",
                "timestamp": "2026-03-15T10:00:00Z",
                "session_id": "sess-1",
                "type": "decision",
                "title": "Minimal entry",
            })
            client = AgentlogClient(socket_path=sock_path)
            client._session_id = "sess-1"

            client.write(type="decision", title="Minimal entry")

            req = daemon.last_request
            entry = req["params"]["entry"]
            assert "body" not in entry
            assert "tags" not in entry
            assert "file_refs" not in entry

    def test_search_serializes_query(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)

            client.query("database migration")

            req = daemon.last_request
            assert req["method"] == "search"
            assert req["params"]["query"] == "database migration"

    def test_query_filter_by_session(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)

            client.log(session="sess-1")

            req = daemon.last_request
            assert req["method"] == "query"
            assert req["params"]["session_id"] == "sess-1"

    def test_query_filter_by_type(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)

            client.log(type="decision")

            req = daemon.last_request
            assert req["method"] == "query"
            assert req["params"]["type"] == "decision"

    def test_query_filter_by_tag(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)

            client.log(tag="infrastructure")

            req = daemon.last_request
            assert req["method"] == "query"
            assert req["params"]["tags"] == ["infrastructure"]

    def test_query_filter_by_file(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)

            client.log(file="main.go")

            req = daemon.last_request
            assert req["method"] == "query"
            assert req["params"]["file_path"] == "main.go"


# ---------------------------------------------------------------------------
# Session management tests
# ---------------------------------------------------------------------------

class TestSessionManagement:
    """Tests for automatic session creation and reuse."""

    def test_auto_creates_session_on_first_write(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_responses([
                # First request: create_session
                {"session_id": "auto-sess-1"},
                # Second request: write
                {
                    "id": "entry-1",
                    "timestamp": "2026-03-15T10:00:00Z",
                    "session_id": "auto-sess-1",
                    "type": "decision",
                    "title": "Test",
                },
            ])
            client = AgentlogClient(socket_path=sock_path)
            assert client.session_id is None

            entry_id = client.write(type="decision", title="Test")

            assert client.session_id == "auto-sess-1"
            assert entry_id == "entry-1"
            assert len(daemon.requests) == 2
            assert daemon.requests[0]["method"] == "create_session"
            assert daemon.requests[1]["method"] == "write"

    def test_reuses_session_across_writes(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response({
                "id": "entry-1",
                "timestamp": "2026-03-15T10:00:00Z",
                "session_id": "existing-sess",
                "type": "decision",
                "title": "Test",
            })
            client = AgentlogClient(socket_path=sock_path)
            client._session_id = "existing-sess"

            client.write(type="decision", title="First")
            client.write(type="decision", title="Second")

            # Both writes should use the same session - no create_session calls.
            assert all(r["method"] == "write" for r in daemon.requests)
            assert all(
                r["params"]["entry"]["session_id"] == "existing-sess"
                for r in daemon.requests
            )

    def test_explicit_session_overrides_auto(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response({
                "id": "entry-1",
                "timestamp": "2026-03-15T10:00:00Z",
                "session_id": "explicit-sess",
                "type": "decision",
                "title": "Test",
            })
            client = AgentlogClient(socket_path=sock_path)
            client._session_id = "auto-sess"

            client.write(type="decision", title="Test", session="explicit-sess")

            req = daemon.last_request
            assert req["params"]["entry"]["session_id"] == "explicit-sess"
            # Auto session should still be the original.
            assert client.session_id == "auto-sess"


# ---------------------------------------------------------------------------
# Error handling tests
# ---------------------------------------------------------------------------

class TestErrorHandling:
    """Tests for error conditions."""

    def test_daemon_not_running_raises(self) -> None:
        client = AgentlogClient(socket_path="/nonexistent/path/test.sock")
        with pytest.raises(DaemonNotRunning, match="daemon socket not found"):
            client._send("create_session")

    def test_daemon_error_response_raises(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_error("something went wrong")
            client = AgentlogClient(socket_path=sock_path)

            with pytest.raises(AgentlogError, match="something went wrong"):
                client._send("bad_method")

    def test_connection_error_raises(self, sock_dir: str) -> None:
        # Create a regular file (not a socket) so os.path.exists returns True
        # but socket.connect fails.
        sock_path = os.path.join(sock_dir, "broken.sock")
        Path(sock_path).touch()

        client = AgentlogClient(socket_path=sock_path)
        with pytest.raises(ConnectionError, match="failed to connect"):
            client._send("create_session")


# ---------------------------------------------------------------------------
# Duration parsing tests
# ---------------------------------------------------------------------------

class TestDurationParsing:
    """Tests for the _parse_duration and _resolve_time helpers."""

    def test_parse_seconds(self) -> None:
        from datetime import timedelta
        assert _parse_duration("30s") == timedelta(seconds=30)

    def test_parse_minutes(self) -> None:
        from datetime import timedelta
        assert _parse_duration("5m") == timedelta(minutes=5)

    def test_parse_hours(self) -> None:
        from datetime import timedelta
        assert _parse_duration("2h") == timedelta(hours=2)

    def test_parse_days(self) -> None:
        from datetime import timedelta
        assert _parse_duration("7d") == timedelta(days=7)

    def test_parse_weeks(self) -> None:
        from datetime import timedelta
        assert _parse_duration("1w") == timedelta(weeks=1)

    def test_parse_invalid_suffix(self) -> None:
        with pytest.raises(ValueError, match="unsupported duration suffix"):
            _parse_duration("5x")

    def test_parse_empty_string(self) -> None:
        with pytest.raises(ValueError, match="empty duration"):
            _parse_duration("")

    def test_resolve_time_passes_through_iso(self) -> None:
        iso = "2026-03-15T10:30:00Z"
        assert _resolve_time(iso) == iso

    def test_resolve_time_resolves_duration(self) -> None:
        result = _resolve_time("1h")
        # Should be a valid ISO-ish string.
        assert "T" in result
        assert result.endswith("Z")


# ---------------------------------------------------------------------------
# Client configuration tests
# ---------------------------------------------------------------------------

class TestClientConfig:
    """Tests for client initialization and configuration."""

    def test_default_socket_path(self) -> None:
        client = AgentlogClient()
        expected = str(Path.home() / ".agentlog" / "agentlogd.sock")
        assert client.socket_path == expected

    def test_custom_agentlog_dir(self) -> None:
        client = AgentlogClient(agentlog_dir="/custom/dir")
        assert client.socket_path == "/custom/dir/agentlogd.sock"

    def test_custom_socket_path_overrides_dir(self) -> None:
        client = AgentlogClient(
            agentlog_dir="/custom/dir",
            socket_path="/other/path.sock",
        )
        assert client.socket_path == "/other/path.sock"

    def test_env_var_sets_dir(self) -> None:
        with patch.dict(os.environ, {"AGENTLOG_DIR": "/env/dir"}):
            client = AgentlogClient()
            assert client.socket_path == "/env/dir/agentlogd.sock"

    def test_explicit_dir_overrides_env(self) -> None:
        with patch.dict(os.environ, {"AGENTLOG_DIR": "/env/dir"}):
            client = AgentlogClient(agentlog_dir="/explicit/dir")
            assert client.socket_path == "/explicit/dir/agentlogd.sock"


# ---------------------------------------------------------------------------
# Context formatting tests
# ---------------------------------------------------------------------------

class TestContextFormatting:
    """Tests for the context() output formatting."""

    def test_format_empty_entries(self) -> None:
        client = AgentlogClient()
        result = client._format_context([])
        assert "No entries found" in result

    def test_format_single_entry(self) -> None:
        client = AgentlogClient()
        entries = [{
            "id": "entry-1",
            "timestamp": "2026-03-15T10:30:00Z",
            "session_id": "sess-1",
            "type": "decision",
            "title": "Use PostgreSQL",
            "body": "Better for relational data.",
            "tags": ["database", "infrastructure"],
            "file_refs": ["config.yaml", "docker-compose.yml"],
        }]
        result = client._format_context(entries)
        assert "# Recent decisions" in result
        assert "## [decision] Use PostgreSQL (2026-03-15 10:30)" in result
        assert "Better for relational data." in result
        assert "Tags: database, infrastructure" in result
        assert "Files: config.yaml, docker-compose.yml" in result

    def test_format_entry_without_optional_fields(self) -> None:
        client = AgentlogClient()
        entries = [{
            "id": "entry-1",
            "timestamp": "2026-03-15T10:30:00Z",
            "session_id": "sess-1",
            "type": "assumption",
            "title": "Users have Python 3.9+",
        }]
        result = client._format_context(entries)
        assert "## [assumption] Users have Python 3.9+" in result
        assert "Tags:" not in result
        assert "Files:" not in result


# ---------------------------------------------------------------------------
# Context daemon call tests
# ---------------------------------------------------------------------------

class TestContextDaemonCall:
    """Tests for context() sending requests to the daemon context method."""

    def test_context_with_files(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([{
                "id": "entry-1",
                "timestamp": "2026-03-15T10:30:00Z",
                "type": "decision",
                "title": "Use Redis",
                "file_refs": ["config/redis.yaml"],
            }])
            client = AgentlogClient(socket_path=sock_path)
            result = client.context(files=["config/redis.yaml"])

            req = daemon.last_request
            assert req["method"] == "context"
            assert req["params"]["files"] == ["config/redis.yaml"]
            assert "topic" not in req["params"]
            assert "# Recent decisions" in result

    def test_context_with_topic(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([{
                "id": "entry-1",
                "timestamp": "2026-03-15T10:30:00Z",
                "type": "decision",
                "title": "Use JWT for auth",
            }])
            client = AgentlogClient(socket_path=sock_path)
            result = client.context(topic="authentication")

            req = daemon.last_request
            assert req["method"] == "context"
            assert req["params"]["topic"] == "authentication"
            assert "files" not in req["params"]
            assert "Use JWT for auth" in result

    def test_context_with_files_and_topic(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)
            client.context(files=["main.go"], topic="auth")

            req = daemon.last_request
            assert req["method"] == "context"
            assert req["params"]["files"] == ["main.go"]
            assert req["params"]["topic"] == "auth"

    def test_context_with_limit(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)
            client.context(topic="test", limit=5)

            req = daemon.last_request
            assert req["method"] == "context"
            assert req["params"]["limit"] == 5

    def test_context_empty_result(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)
            result = client.context(topic="nonexistent")

            assert "No entries found" in result


# ---------------------------------------------------------------------------
# Client-side filtering tests
# ---------------------------------------------------------------------------

class TestClientSideFiltering:
    """Tests for _filter_entries used in query() and log()."""

    def test_filter_by_type(self) -> None:
        entries = [
            {"type": "decision", "title": "A"},
            {"type": "assumption", "title": "B"},
            {"type": "decision", "title": "C"},
        ]
        result = AgentlogClient._filter_entries(entries, type="decision")
        assert len(result) == 2
        assert all(e["type"] == "decision" for e in result)

    def test_filter_by_tag(self) -> None:
        entries = [
            {"title": "A", "tags": ["db", "infra"]},
            {"title": "B", "tags": ["api"]},
            {"title": "C"},
        ]
        result = AgentlogClient._filter_entries(entries, tag="db")
        assert len(result) == 1
        assert result[0]["title"] == "A"

    def test_filter_by_file(self) -> None:
        entries = [
            {"title": "A", "file_refs": ["main.go"]},
            {"title": "B", "file_refs": ["config.yaml"]},
            {"title": "C"},
        ]
        result = AgentlogClient._filter_entries(entries, file="main.go")
        assert len(result) == 1
        assert result[0]["title"] == "A"

    def test_filter_by_session(self) -> None:
        entries = [
            {"title": "A", "session_id": "s1"},
            {"title": "B", "session_id": "s2"},
        ]
        result = AgentlogClient._filter_entries(entries, session="s1")
        assert len(result) == 1
        assert result[0]["title"] == "A"

    def test_limit_applied_in_query(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([
                {"title": f"Entry {i}", "type": "decision"} for i in range(10)
            ])
            client = AgentlogClient(socket_path=sock_path)
            results = client.query("test", limit=3)
            assert len(results) == 3


# ---------------------------------------------------------------------------
# Log method tests
# ---------------------------------------------------------------------------

class TestLogMethod:
    """Tests for the log() method default behavior."""

    def test_log_no_filters_defaults_to_time_range(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)
            client.log()

            req = daemon.last_request
            assert req["method"] == "query"
            assert "start" in req["params"]
            assert "end" in req["params"]

    def test_log_with_offset(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([
                {"title": f"Entry {i}", "type": "decision"} for i in range(10)
            ])
            client = AgentlogClient(socket_path=sock_path)
            results = client.log(type="decision", limit=3, offset=2)
            assert len(results) == 3
            assert results[0]["title"] == "Entry 2"

    def test_log_since_duration(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response([])
            client = AgentlogClient(socket_path=sock_path)
            client.log(since="1h")

            req = daemon.last_request
            assert req["method"] == "query"
            assert "start" in req["params"]
            assert "end" in req["params"]


# ---------------------------------------------------------------------------
# Export method tests
# ---------------------------------------------------------------------------

class TestExportMethod:
    """Tests for the export() method."""

    def test_basic_export(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("# Decision Log Export\n\n## Use Redis\n")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export()

            req = daemon.last_request
            assert req["method"] == "export"
            assert result == "# Decision Log Export\n\n## Use Redis\n"

    def test_export_with_session(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(session="sess-123")

            req = daemon.last_request
            assert req["method"] == "export"
            assert req["params"]["session_id"] == "sess-123"

    def test_export_with_since_duration(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(since="7d")

            req = daemon.last_request
            assert req["method"] == "export"
            # Duration should be resolved to ISO 8601 string.
            assert "T" in req["params"]["since"]
            assert req["params"]["since"].endswith("Z")

    def test_export_with_until_duration(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(until="1h")

            req = daemon.last_request
            assert req["method"] == "export"
            assert "T" in req["params"]["until"]
            assert req["params"]["until"].endswith("Z")

    def test_export_with_since_iso(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(since="2026-03-01T00:00:00Z")

            req = daemon.last_request
            assert req["params"]["since"] == "2026-03-01T00:00:00Z"

    def test_export_with_file(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(file="main.go")

            req = daemon.last_request
            assert req["params"]["file_path"] == "main.go"

    def test_export_with_tag(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(tag="infrastructure")

            req = daemon.last_request
            assert req["params"]["tag"] == "infrastructure"

    def test_export_with_type(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(type="decision")

            req = daemon.last_request
            assert req["params"]["type"] == "decision"

    def test_export_format_json(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("[]")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export(format="json")

            req = daemon.last_request
            assert req["params"]["format"] == "json"
            assert result == "[]"

    def test_export_format_text(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("No entries found.")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export(format="text")

            req = daemon.last_request
            assert req["params"]["format"] == "text"
            assert result == "No entries found."

    def test_export_format_markdown(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("# Decision Log Export\n")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export(format="markdown")

            req = daemon.last_request
            assert req["params"]["format"] == "markdown"

    def test_export_template_pr(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("## What changed\n\n- **Use Redis**\n")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export(template="pr")

            req = daemon.last_request
            assert req["params"]["template"] == "pr"
            assert "What changed" in result

    def test_export_template_retro(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("# Retrospective\n")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export(template="retro")

            req = daemon.last_request
            assert req["params"]["template"] == "retro"

    def test_export_template_handoff(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("# Handoff Document\n")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export(template="handoff")

            req = daemon.last_request
            assert req["params"]["template"] == "handoff"

    def test_export_empty_result(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("No entries found.")
            client = AgentlogClient(socket_path=sock_path)
            result = client.export()

            assert result == "No entries found."

    def test_export_omits_none_params(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(tag="db")

            req = daemon.last_request
            assert req["method"] == "export"
            assert req["params"] == {"tag": "db"}

    def test_export_all_params(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response("exported")
            client = AgentlogClient(socket_path=sock_path)
            client.export(
                session="sess-1",
                since="2026-03-01T00:00:00Z",
                until="2026-03-15T00:00:00Z",
                file="main.go",
                tag="db",
                type="decision",
                format="text",
                template="pr",
            )

            req = daemon.last_request
            assert req["params"]["session_id"] == "sess-1"
            assert req["params"]["since"] == "2026-03-01T00:00:00Z"
            assert req["params"]["until"] == "2026-03-15T00:00:00Z"
            assert req["params"]["file_path"] == "main.go"
            assert req["params"]["tag"] == "db"
            assert req["params"]["type"] == "decision"
            assert req["params"]["format"] == "text"
            assert req["params"]["template"] == "pr"

    def test_export_returns_empty_string_for_non_string_result(self, sock_path: str) -> None:
        with FakeDaemon(sock_path) as daemon:
            daemon.set_response(42)
            client = AgentlogClient(socket_path=sock_path)
            result = client.export()

            assert result == ""
