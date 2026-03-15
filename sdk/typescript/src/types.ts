/**
 * TypeScript type definitions for the agentlog SDK.
 */

/** Valid entry types for decision log entries. */
export const VALID_ENTRY_TYPES = [
  "decision",
  "attempt_failed",
  "deferred",
  "assumption",
  "question",
] as const;

export type EntryType = (typeof VALID_ENTRY_TYPES)[number];

/** Options for writing an entry. */
export interface WriteOptions {
  /** Entry type. Must be one of the valid entry types. */
  type: EntryType;
  /** Short summary of the decision. */
  title: string;
  /** Optional longer description. */
  body?: string;
  /** Optional list of tags. */
  tags?: string[];
  /** Optional list of file references. */
  files?: string[];
  /** Optional session ID. If not provided, the client auto-creates one. */
  session?: string;
}

/** Options for full-text search. */
export interface QueryOptions {
  /** Search query string. */
  text: string;
  /** Filter results by entry type. */
  type?: EntryType;
  /** Filter results by session ID. */
  session?: string;
  /** Filter results by tag. */
  tag?: string;
  /** Filter results by file reference. */
  file?: string;
  /** Only return entries after this time (ISO 8601 or duration like "1h"). */
  since?: string;
  /** Only return entries before this time (ISO 8601 or duration like "1h"). */
  until?: string;
  /** Maximum number of results to return. Defaults to 20. */
  limit?: number;
}

/** Options for listing entries with filters. */
export interface LogOptions {
  /** Filter by entry type. */
  type?: EntryType;
  /** Filter by session ID. */
  session?: string;
  /** Filter by tag. */
  tag?: string;
  /** Filter by file reference. */
  file?: string;
  /** Only entries after this time (ISO 8601 or duration like "1h", "7d"). */
  since?: string;
  /** Only entries before this time (ISO 8601 or duration like "1h", "7d"). */
  until?: string;
  /** Maximum number of entries to return. Defaults to 50. */
  limit?: number;
  /** Number of entries to skip (for pagination). Defaults to 0. */
  offset?: number;
}

/** Options for context retrieval. */
export interface ContextOptions {
  /** Optional search query for full-text search. */
  query?: string;
  /** Optional session ID to fetch entries from. */
  session?: string;
  /** Maximum number of entries to include. Defaults to 10. */
  limit?: number;
}

/** Options for constructing an AgentlogClient. */
export interface ClientOptions {
  /**
   * Path to the agentlog data directory. If not provided, uses the
   * AGENTLOG_DIR environment variable or defaults to ~/.agentlog.
   */
  agentlogDir?: string;
  /**
   * Explicit path to the daemon socket. Overrides the default derived
   * from agentlogDir.
   */
  socketPath?: string;
}

/** A log entry as returned by the daemon. */
export interface Entry {
  id: string;
  timestamp: string;
  session_id: string;
  type: string;
  title: string;
  body?: string;
  tags?: string[];
  file_refs?: string[];
}

/** Daemon request envelope. */
export interface DaemonRequest {
  method: string;
  params?: Record<string, unknown>;
}

/** Daemon response envelope. */
export interface DaemonResponse {
  ok: boolean;
  result?: unknown;
  error?: string;
}
