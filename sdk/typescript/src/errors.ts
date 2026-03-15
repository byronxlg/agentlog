/**
 * Exception hierarchy for the agentlog TypeScript SDK.
 */

/** Base error for all agentlog SDK errors. */
export class AgentlogError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "AgentlogError";
  }
}

/** Raised when the SDK cannot connect to the daemon socket. */
export class ConnectionError extends AgentlogError {
  constructor(message: string) {
    super(message);
    this.name = "ConnectionError";
  }
}

/** Raised when the daemon socket does not exist, indicating the daemon is not running. */
export class DaemonNotRunningError extends AgentlogError {
  constructor(message: string) {
    super(message);
    this.name = "DaemonNotRunningError";
  }
}
