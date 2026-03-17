"""Integration tests for the agentlog Python SDK.

These tests require a running agentlog daemon. They are automatically skipped
if the daemon socket does not exist.
"""

from __future__ import annotations

import os
from pathlib import Path

import pytest

import agentlog
from agentlog import AgentlogClient

# Determine socket path from env or default.
_agentlog_dir = os.environ.get("AGENTLOG_DIR", str(Path.home() / ".agentlog"))
_socket_path = os.path.join(_agentlog_dir, "agentlogd.sock")

pytestmark = pytest.mark.skipif(
    not os.path.exists(_socket_path),
    reason=f"daemon not running (socket not found at {_socket_path})",
)


@pytest.fixture
def client() -> AgentlogClient:
    """Create a fresh client for each test."""
    return AgentlogClient()


class TestIntegrationWrite:
    """Integration tests for writing entries."""

    def test_write_and_query(self, client: AgentlogClient) -> None:
        """Write an entry, then search for it by title text."""
        entry_id = client.write(
            type="decision",
            title="Integration test: use SQLite for local storage",
            body="SQLite is lightweight and requires no separate process.",
            tags=["integration-test", "database"],
            files=["internal/store/store.go"],
        )

        assert entry_id
        assert isinstance(entry_id, str)

        # Search for the entry we just wrote.
        results = client.query("SQLite local storage")
        assert len(results) > 0

        # At least one result should match our entry.
        ids = [r["id"] for r in results]
        assert entry_id in ids

    def test_write_returns_different_ids(self, client: AgentlogClient) -> None:
        """Each write should return a unique entry ID."""
        id1 = client.write(type="assumption", title="Integration test: unique ID 1")
        id2 = client.write(type="assumption", title="Integration test: unique ID 2")
        assert id1 != id2


class TestIntegrationSession:
    """Integration tests for session management."""

    def test_auto_session_created(self, client: AgentlogClient) -> None:
        """First write should create a session automatically."""
        assert client.session_id is None

        client.write(type="decision", title="Integration test: auto session")

        assert client.session_id is not None
        assert isinstance(client.session_id, str)

    def test_session_reused_across_writes(self, client: AgentlogClient) -> None:
        """Subsequent writes should reuse the auto-created session."""
        client.write(type="decision", title="Integration test: session reuse 1")
        first_session = client.session_id

        client.write(type="decision", title="Integration test: session reuse 2")
        assert client.session_id == first_session


class TestIntegrationLog:
    """Integration tests for listing entries."""

    def test_log_by_type(self, client: AgentlogClient) -> None:
        """Write an entry and retrieve it by type filter."""
        client.write(type="question", title="Integration test: log by type")

        results = client.log(type="question")
        assert len(results) > 0
        assert all(r["type"] == "question" for r in results)

    def test_log_by_session(self, client: AgentlogClient) -> None:
        """Write entries in a session and retrieve them."""
        client.write(type="decision", title="Integration test: log by session 1")
        client.write(type="assumption", title="Integration test: log by session 2")
        session_id = client.session_id

        results = client.log(session=session_id)
        assert len(results) >= 2


class TestIntegrationContext:
    """Integration tests for the context() method."""

    def test_context_with_topic(self, client: AgentlogClient) -> None:
        """context(topic=...) should return matching entries."""
        client.write(
            type="decision",
            title="Integration test: context topic search",
            body="This entry tests topic-based context retrieval.",
            tags=["integration-test"],
        )

        result = client.context(topic="context topic search")
        assert "# Recent decisions" in result
        assert "context topic search" in result

    def test_context_with_files(self, client: AgentlogClient) -> None:
        """context(files=...) should return entries related to those files."""
        client.write(
            type="decision",
            title="Integration test: context file lookup",
            body="This entry tests file-based context retrieval.",
            files=["sdk/python/tests/test_integration.py"],
        )

        result = client.context(files=["sdk/python/tests/test_integration.py"])
        assert "# Recent decisions" in result
        assert "context file lookup" in result

    def test_context_with_files_and_topic(self, client: AgentlogClient) -> None:
        """context(files=..., topic=...) should combine both criteria."""
        client.write(
            type="decision",
            title="Integration test: context combined",
            body="This entry tests combined context retrieval.",
            files=["internal/combined_test.go"],
        )

        result = client.context(
            files=["internal/combined_test.go"],
            topic="context combined",
        )
        assert "# Recent decisions" in result

    def test_context_empty_result(self, client: AgentlogClient) -> None:
        """context() with no matching topic should return no entries."""
        result = client.context(topic="xyzzy_nonexistent_topic_12345")
        assert "No entries found" in result


class TestIntegrationModuleFunctions:
    """Integration tests for module-level convenience functions."""

    def test_module_write(self) -> None:
        """Module-level write() should work without explicit client creation."""
        entry_id = agentlog.write(
            type="decision",
            title="Integration test: module-level write",
        )
        assert entry_id
        assert isinstance(entry_id, str)
