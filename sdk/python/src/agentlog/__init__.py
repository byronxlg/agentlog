"""agentlog - Python SDK for the agentlog decision log daemon.

Usage::

    import agentlog

    agentlog.write("decision", "Use PostgreSQL for persistence")
    entries = agentlog.query("database")
"""

from __future__ import annotations

from .client import AgentlogClient, VALID_ENTRY_TYPES
from .errors import AgentlogError, ConnectionError, DaemonNotRunning

__all__ = [
    "AgentlogClient",
    "AgentlogError",
    "ConnectionError",
    "DaemonNotRunning",
    "VALID_ENTRY_TYPES",
    "write",
    "query",
    "log",
    "context",
    "export",
]

_default_client: AgentlogClient | None = None


def _get_default_client() -> AgentlogClient:
    """Return the module-level default client, creating it on first use."""
    global _default_client
    if _default_client is None:
        _default_client = AgentlogClient()
    return _default_client


def write(
    type: str,
    title: str,
    body: str | None = None,
    tags: list[str] | None = None,
    files: list[str] | None = None,
    session: str | None = None,
) -> str:
    """Write a decision entry to the log.

    Convenience wrapper around :meth:`AgentlogClient.write` using the
    module-level default client.

    Args:
        type: Entry type (decision, attempt_failed, deferred, assumption, question).
        title: Short summary of the decision.
        body: Optional longer description.
        tags: Optional list of tags.
        files: Optional list of file references.
        session: Optional session ID.

    Returns:
        The ID of the written entry.
    """
    return _get_default_client().write(
        type=type, title=title, body=body, tags=tags, files=files, session=session,
    )


def query(
    text: str,
    type: str | None = None,
    session: str | None = None,
    tag: str | None = None,
    file: str | None = None,
    since: str | None = None,
    until: str | None = None,
    limit: int = 20,
) -> list[dict]:
    """Full-text search for entries.

    Convenience wrapper around :meth:`AgentlogClient.query`.
    """
    return _get_default_client().query(
        text=text, type=type, session=session, tag=tag, file=file,
        since=since, until=until, limit=limit,
    )


def log(
    type: str | None = None,
    session: str | None = None,
    tag: str | None = None,
    file: str | None = None,
    since: str | None = None,
    until: str | None = None,
    limit: int = 50,
    offset: int = 0,
) -> list[dict]:
    """List entries with filters.

    Convenience wrapper around :meth:`AgentlogClient.log`.
    """
    return _get_default_client().log(
        type=type, session=session, tag=tag, file=file,
        since=since, until=until, limit=limit, offset=offset,
    )


def context(
    files: list[str] | None = None,
    topic: str | None = None,
    limit: int | None = None,
) -> str:
    """Return a structured context string suitable for prompt injection.

    Convenience wrapper around :meth:`AgentlogClient.context`.
    """
    return _get_default_client().context(files=files, topic=topic, limit=limit)


def export(
    session: str | None = None,
    since: str | None = None,
    until: str | None = None,
    file: str | None = None,
    tag: str | None = None,
    type: str | None = None,
    format: str | None = None,
    template: str | None = None,
) -> str:
    """Export entries as a formatted string.

    Convenience wrapper around :meth:`AgentlogClient.export`.
    """
    return _get_default_client().export(
        session=session, since=since, until=until, file=file,
        tag=tag, type=type, format=format, template=template,
    )
