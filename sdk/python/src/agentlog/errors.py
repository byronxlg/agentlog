"""Exception hierarchy for the agentlog SDK."""

from __future__ import annotations


class AgentlogError(Exception):
    """Base exception for all agentlog SDK errors."""


class ConnectionError(AgentlogError):
    """Raised when the SDK cannot connect to the daemon socket."""


class DaemonNotRunning(AgentlogError):
    """Raised when the daemon socket does not exist, indicating the daemon is not running."""
