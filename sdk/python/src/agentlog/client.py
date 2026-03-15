"""AgentlogClient - thin client wrapping the agentlog daemon's Unix socket protocol."""

from __future__ import annotations

import json
import os
import socket
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any, Optional

from .errors import AgentlogError, ConnectionError, DaemonNotRunning

# Allowed entry types for validation.
VALID_ENTRY_TYPES = frozenset({
    "decision",
    "attempt_failed",
    "deferred",
    "assumption",
    "question",
})


def _parse_duration(value: str) -> timedelta:
    """Parse a relative duration string like '1h', '7d', '30m' into a timedelta.

    Supported suffixes: s (seconds), m (minutes), h (hours), d (days), w (weeks).
    """
    if not value:
        raise ValueError("empty duration string")

    suffix = value[-1].lower()
    try:
        amount = int(value[:-1])
    except ValueError:
        raise ValueError(f"invalid duration: {value!r}") from None

    multipliers = {
        "s": timedelta(seconds=1),
        "m": timedelta(minutes=1),
        "h": timedelta(hours=1),
        "d": timedelta(days=1),
        "w": timedelta(weeks=1),
    }
    if suffix not in multipliers:
        raise ValueError(
            f"unsupported duration suffix {suffix!r} in {value!r}; "
            f"use one of: s, m, h, d, w"
        )
    return amount * multipliers[suffix]


def _resolve_time(value: str) -> str:
    """Resolve a time value to an ISO 8601 / RFC 3339 string.

    Accepts either an ISO 8601 datetime string (passed through) or a relative
    duration like '1h', '7d' (resolved relative to now).
    """
    # If it looks like a relative duration (digits followed by a letter), parse it.
    stripped = value.strip()
    if stripped and stripped[-1].isalpha() and stripped[:-1].isdigit():
        delta = _parse_duration(stripped)
        resolved = datetime.now(timezone.utc) - delta
        return resolved.strftime("%Y-%m-%dT%H:%M:%S.%fZ")
    # Otherwise treat as an ISO 8601 string and pass through.
    return stripped


class AgentlogClient:
    """Client for communicating with the agentlog daemon over a Unix socket.

    Args:
        agentlog_dir: Path to the agentlog data directory. If not provided,
            uses the ``AGENTLOG_DIR`` environment variable or defaults to
            ``~/.agentlog``.
        socket_path: Explicit path to the daemon socket. Overrides the
            default derived from ``agentlog_dir``.
    """

    def __init__(
        self,
        agentlog_dir: Optional[str] = None,
        socket_path: Optional[str] = None,
    ) -> None:
        if agentlog_dir is None:
            agentlog_dir = os.environ.get("AGENTLOG_DIR")
        if agentlog_dir is None:
            agentlog_dir = str(Path.home() / ".agentlog")

        self._agentlog_dir = agentlog_dir

        if socket_path is not None:
            self._socket_path = socket_path
        else:
            self._socket_path = os.path.join(agentlog_dir, "agentlogd.sock")

        self._session_id: Optional[str] = None

    @property
    def socket_path(self) -> str:
        """The Unix socket path this client connects to."""
        return self._socket_path

    @property
    def session_id(self) -> Optional[str]:
        """The current session ID, or None if no session has been created yet."""
        return self._session_id

    # -- low-level transport ------------------------------------------------

    def _send(self, method: str, params: Optional[dict[str, Any]] = None) -> Any:
        """Send a request to the daemon and return the result.

        Opens a new Unix socket connection, sends a single JSON line, reads a
        single JSON line response, and closes the connection.

        Raises:
            DaemonNotRunning: If the socket file does not exist.
            ConnectionError: If the connection to the daemon fails.
            AgentlogError: If the daemon returns an error response.
        """
        if not os.path.exists(self._socket_path):
            raise DaemonNotRunning(
                f"daemon socket not found at {self._socket_path}; "
                f"is agentlogd running?"
            )

        request: dict[str, Any] = {"method": method}
        if params is not None:
            request["params"] = params

        try:
            sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            sock.connect(self._socket_path)
        except OSError as exc:
            raise ConnectionError(
                f"failed to connect to daemon at {self._socket_path}: {exc}"
            ) from exc

        try:
            payload = json.dumps(request) + "\n"
            sock.sendall(payload.encode("utf-8"))

            # Read response - accumulate data until we get a newline.
            buf = b""
            while b"\n" not in buf:
                chunk = sock.recv(4096)
                if not chunk:
                    break
                buf += chunk
        finally:
            sock.close()

        if not buf.strip():
            raise AgentlogError("empty response from daemon")

        try:
            response = json.loads(buf)
        except json.JSONDecodeError as exc:
            raise AgentlogError(f"invalid JSON response from daemon: {exc}") from exc

        if not response.get("ok"):
            error_msg = response.get("error", "unknown error")
            raise AgentlogError(f"daemon error: {error_msg}")

        return response.get("result")

    # -- session management -------------------------------------------------

    def _ensure_session(self) -> str:
        """Return the current session ID, creating one if necessary."""
        if self._session_id is None:
            result = self._send("create_session")
            self._session_id = result["session_id"]
        return self._session_id

    # -- public API ---------------------------------------------------------

    def write(
        self,
        type: str,
        title: str,
        body: Optional[str] = None,
        tags: Optional[list[str]] = None,
        files: Optional[list[str]] = None,
        session: Optional[str] = None,
    ) -> str:
        """Write a decision entry to the log.

        Args:
            type: Entry type. Must be one of: ``decision``, ``attempt_failed``,
                ``deferred``, ``assumption``, ``question``.
            title: Short summary of the decision.
            body: Optional longer description.
            tags: Optional list of tags.
            files: Optional list of file references.
            session: Optional session ID. If not provided, the client creates a
                session automatically on first write and reuses it.

        Returns:
            The ID of the written entry.

        Raises:
            ValueError: If ``type`` is not a valid entry type.
            AgentlogError: On daemon communication errors.
        """
        if type not in VALID_ENTRY_TYPES:
            raise ValueError(
                f"invalid entry type {type!r}; must be one of: "
                f"{', '.join(sorted(VALID_ENTRY_TYPES))}"
            )

        session_id = session if session is not None else self._ensure_session()

        entry: dict[str, Any] = {
            "session_id": session_id,
            "type": type,
            "title": title,
        }
        if body is not None:
            entry["body"] = body
        if tags:
            entry["tags"] = tags
        if files:
            entry["file_refs"] = files

        result = self._send("write", {"entry": entry})
        return result["id"]

    def query(
        self,
        text: str,
        type: Optional[str] = None,
        session: Optional[str] = None,
        tag: Optional[str] = None,
        file: Optional[str] = None,
        since: Optional[str] = None,
        until: Optional[str] = None,
        limit: int = 20,
    ) -> list[dict[str, Any]]:
        """Full-text search for entries.

        Args:
            text: Search query string.
            type: Filter results by entry type.
            session: Filter results by session ID.
            tag: Filter results by tag.
            file: Filter results by file reference.
            since: Only return entries after this time (ISO 8601 or duration like '1h').
            until: Only return entries before this time (ISO 8601 or duration like '1h').
            limit: Maximum number of results to return.

        Returns:
            List of entry dicts matching the search, in relevance order.
        """
        result = self._send("search", {"query": text})
        entries = result if isinstance(result, list) else []

        entries = self._filter_entries(
            entries, type=type, session=session, tag=tag, file=file,
            since=since, until=until,
        )

        return entries[:limit]

    def log(
        self,
        type: Optional[str] = None,
        session: Optional[str] = None,
        tag: Optional[str] = None,
        file: Optional[str] = None,
        since: Optional[str] = None,
        until: Optional[str] = None,
        limit: int = 50,
        offset: int = 0,
    ) -> list[dict[str, Any]]:
        """List entries with filters.

        At least one filter must be provided, or the method defaults to
        entries from the last 24 hours.

        Args:
            type: Filter by entry type.
            session: Filter by session ID.
            tag: Filter by tag.
            file: Filter by file reference.
            since: Only entries after this time (ISO 8601 or duration like '1h', '7d').
            until: Only entries before this time (ISO 8601 or duration like '1h', '7d').
            limit: Maximum number of entries to return.
            offset: Number of entries to skip (for pagination).

        Returns:
            List of entry dicts matching the filters, sorted by timestamp.
        """
        has_filter = any([type, session, tag, file, since, until])

        # Choose the most appropriate daemon query method.
        params: dict[str, Any] = {}
        remaining_filters: dict[str, Any] = {}

        if session:
            params["session_id"] = session
            remaining_filters = {"type": type, "tag": tag, "file": file,
                                 "since": since, "until": until}
        elif type:
            params["type"] = type
            remaining_filters = {"session": session, "tag": tag, "file": file,
                                 "since": since, "until": until}
        elif tag:
            params["tags"] = [tag]
            remaining_filters = {"type": type, "session": session, "file": file,
                                 "since": since, "until": until}
        elif file:
            params["file_path"] = file
            remaining_filters = {"type": type, "session": session, "tag": tag,
                                 "since": since, "until": until}
        elif since or until:
            # Use time range query.
            start = _resolve_time(since) if since else "1970-01-01T00:00:00Z"
            end = _resolve_time(until) if until else datetime.now(timezone.utc).strftime(
                "%Y-%m-%dT%H:%M:%S.%fZ"
            )
            params["start"] = start
            params["end"] = end
            remaining_filters = {"type": type, "session": session, "tag": tag,
                                 "file": file}
        else:
            # No filters - default to last 24 hours.
            now = datetime.now(timezone.utc)
            start = (now - timedelta(hours=24)).strftime("%Y-%m-%dT%H:%M:%S.%fZ")
            end = now.strftime("%Y-%m-%dT%H:%M:%S.%fZ")
            params["start"] = start
            params["end"] = end

        result = self._send("query", params)
        entries = result if isinstance(result, list) else []

        # Apply any remaining client-side filters.
        entries = self._filter_entries(entries, **remaining_filters)

        # Apply offset and limit.
        entries = entries[offset:offset + limit]

        return entries

    def context(
        self,
        query: Optional[str] = None,
        session: Optional[str] = None,
        limit: int = 10,
    ) -> str:
        """Return a structured context string suitable for prompt injection.

        Fetches entries via search or session query and formats them as a
        readable text block.

        Args:
            query: Optional search query for full-text search.
            session: Optional session ID to fetch entries from.
            limit: Maximum number of entries to include.

        Returns:
            A formatted string with entry summaries.
        """
        if query:
            entries = self.query(query, limit=limit)
        elif session:
            result = self._send("get_session", {"session_id": session})
            entries = result if isinstance(result, list) else []
            entries = entries[:limit]
        else:
            entries = self.log(limit=limit)

        return self._format_context(entries)

    # -- internal helpers ---------------------------------------------------

    @staticmethod
    def _filter_entries(
        entries: list[dict[str, Any]],
        type: Optional[str] = None,
        session: Optional[str] = None,
        tag: Optional[str] = None,
        file: Optional[str] = None,
        since: Optional[str] = None,
        until: Optional[str] = None,
    ) -> list[dict[str, Any]]:
        """Apply client-side filters to a list of entry dicts."""
        filtered = entries

        if type:
            filtered = [e for e in filtered if e.get("type") == type]
        if session:
            filtered = [e for e in filtered if e.get("session_id") == session]
        if tag:
            filtered = [e for e in filtered if tag in (e.get("tags") or [])]
        if file:
            filtered = [e for e in filtered if file in (e.get("file_refs") or [])]
        if since:
            since_dt = _resolve_time(since)
            filtered = [e for e in filtered if e.get("timestamp", "") >= since_dt]
        if until:
            until_dt = _resolve_time(until)
            filtered = [e for e in filtered if e.get("timestamp", "") <= until_dt]

        return filtered

    @staticmethod
    def _format_context(entries: list[dict[str, Any]]) -> str:
        """Format entries into a structured text block for prompt injection."""
        if not entries:
            return "# Recent decisions\n\nNo entries found."

        lines = ["# Recent decisions", ""]
        for entry in entries:
            entry_type = entry.get("type", "unknown")
            title = entry.get("title", "Untitled")
            timestamp = entry.get("timestamp", "")
            # Format timestamp for display - truncate to minute precision.
            if "T" in timestamp:
                display_time = timestamp[:16].replace("T", " ")
            else:
                display_time = timestamp

            lines.append(f"## [{entry_type}] {title} ({display_time})")

            body = entry.get("body")
            if body:
                lines.append(body)

            tags = entry.get("tags")
            if tags:
                lines.append(f"Tags: {', '.join(tags)}")

            file_refs = entry.get("file_refs")
            if file_refs:
                lines.append(f"Files: {', '.join(file_refs)}")

            lines.append("")

        return "\n".join(lines)
