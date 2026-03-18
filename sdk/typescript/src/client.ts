/**
 * AgentlogClient - thin client wrapping the agentlog daemon's Unix socket protocol.
 */

import { createConnection } from "node:net";
import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";

import { AgentlogError, ConnectionError, DaemonNotRunningError } from "./errors.js";
import type {
  ClientOptions,
  ContextOptions,
  DaemonRequest,
  DaemonResponse,
  Entry,
  ExportOptions,
  LogOptions,
  QueryOptions,
  WriteOptions,
} from "./types.js";
import { VALID_ENTRY_TYPES } from "./types.js";

/**
 * Parse a relative duration string like "1h", "7d", "30m" into milliseconds.
 *
 * Supported suffixes: s (seconds), m (minutes), h (hours), d (days), w (weeks).
 */
export function parseDuration(value: string): number {
  if (!value) {
    throw new Error("empty duration string");
  }

  const suffix = value.slice(-1).toLowerCase();
  const amount = Number(value.slice(0, -1));

  if (Number.isNaN(amount) || !Number.isInteger(amount)) {
    throw new Error(`invalid duration: "${value}"`);
  }

  const multipliers: Record<string, number> = {
    s: 1_000,
    m: 60_000,
    h: 3_600_000,
    d: 86_400_000,
    w: 604_800_000,
  };

  if (!(suffix in multipliers)) {
    throw new Error(
      `unsupported duration suffix "${suffix}" in "${value}"; use one of: s, m, h, d, w`
    );
  }

  return amount * multipliers[suffix];
}

/**
 * Resolve a time value to an ISO 8601 / RFC 3339 string.
 *
 * Accepts either an ISO 8601 datetime string (passed through) or a relative
 * duration like "1h", "7d" (resolved relative to now).
 */
export function resolveTime(value: string): string {
  const stripped = value.trim();
  // Check if it looks like a relative duration: digits followed by a letter.
  if (stripped.length >= 2 && /^\d+[a-zA-Z]$/.test(stripped)) {
    const ms = parseDuration(stripped);
    const resolved = new Date(Date.now() - ms);
    return resolved.toISOString();
  }
  // Otherwise treat as an ISO 8601 string and pass through.
  return stripped;
}

/**
 * Client for communicating with the agentlog daemon over a Unix socket.
 *
 * Each method call opens a new socket connection, sends one JSON-line request,
 * reads one JSON-line response, and closes the connection.
 */
export class AgentlogClient {
  private readonly _socketPath: string;
  private _sessionId: string | null = null;

  /**
   * Create a new AgentlogClient.
   *
   * @param options - Client configuration options.
   */
  constructor(options: ClientOptions = {}) {
    let agentlogDir = options.agentlogDir ?? process.env.AGENTLOG_DIR;
    if (agentlogDir == null) {
      agentlogDir = join(homedir(), ".agentlog");
    }

    if (options.socketPath != null) {
      this._socketPath = options.socketPath;
    } else {
      this._socketPath = join(agentlogDir, "agentlogd.sock");
    }
  }

  /** The Unix socket path this client connects to. */
  get socketPath(): string {
    return this._socketPath;
  }

  /** The current session ID, or null if no session has been created yet. */
  get sessionId(): string | null {
    return this._sessionId;
  }

  // -- low-level transport --------------------------------------------------

  /**
   * Send a request to the daemon and return the result.
   *
   * Opens a new Unix socket connection, sends a single JSON line, reads a
   * single JSON line response, and closes the connection.
   *
   * @throws DaemonNotRunningError if the socket file does not exist.
   * @throws ConnectionError if the connection to the daemon fails.
   * @throws AgentlogError if the daemon returns an error response.
   */
  async _send(
    method: string,
    params?: Record<string, unknown>
  ): Promise<unknown> {
    if (!existsSync(this._socketPath)) {
      throw new DaemonNotRunningError(
        `daemon socket not found at ${this._socketPath}; is agentlogd running?`
      );
    }

    const request: DaemonRequest = { method };
    if (params != null) {
      request.params = params;
    }

    return new Promise<unknown>((resolve, reject) => {
      const socket = createConnection({ path: this._socketPath });
      let buffer = "";

      socket.on("connect", () => {
        const payload = JSON.stringify(request) + "\n";
        socket.write(payload);
      });

      socket.on("data", (chunk: Buffer) => {
        buffer += chunk.toString("utf-8");
        if (buffer.includes("\n")) {
          socket.destroy();
        }
      });

      socket.on("end", () => {
        handleResponse();
      });

      socket.on("close", () => {
        handleResponse();
      });

      socket.on("error", (err: Error) => {
        reject(
          new ConnectionError(
            `failed to connect to daemon at ${this._socketPath}: ${err.message}`
          )
        );
      });

      let handled = false;
      const handleResponse = (): void => {
        if (handled) return;
        handled = true;

        const trimmed = buffer.trim();
        if (!trimmed) {
          reject(new AgentlogError("empty response from daemon"));
          return;
        }

        let response: DaemonResponse;
        try {
          response = JSON.parse(trimmed) as DaemonResponse;
        } catch (err) {
          reject(
            new AgentlogError(
              `invalid JSON response from daemon: ${(err as Error).message}`
            )
          );
          return;
        }

        if (!response.ok) {
          reject(
            new AgentlogError(
              `daemon error: ${response.error ?? "unknown error"}`
            )
          );
          return;
        }

        resolve(response.result);
      };
    });
  }

  // -- session management ---------------------------------------------------

  /**
   * Return the current session ID, creating one if necessary.
   */
  private async _ensureSession(): Promise<string> {
    if (this._sessionId == null) {
      const result = (await this._send("create_session")) as {
        session_id: string;
      };
      this._sessionId = result.session_id;
    }
    return this._sessionId;
  }

  // -- public API -----------------------------------------------------------

  /**
   * Write a decision entry to the log.
   *
   * @param options - Entry fields to write.
   * @returns The ID of the written entry.
   * @throws Error if the entry type is invalid.
   * @throws AgentlogError on daemon communication errors.
   */
  async write(options: WriteOptions): Promise<string> {
    const { type, title, body, tags, files, session } = options;

    if (!VALID_ENTRY_TYPES.includes(type)) {
      throw new Error(
        `invalid entry type "${type}"; must be one of: ${[...VALID_ENTRY_TYPES].sort().join(", ")}`
      );
    }

    const sessionId = session ?? (await this._ensureSession());

    const entry: Record<string, unknown> = {
      session_id: sessionId,
      type,
      title,
    };
    if (body != null) {
      entry.body = body;
    }
    if (tags != null && tags.length > 0) {
      entry.tags = tags;
    }
    if (files != null && files.length > 0) {
      entry.file_refs = files;
    }

    const result = (await this._send("write", { entry })) as { id: string };
    return result.id;
  }

  /**
   * Full-text search for entries.
   *
   * @param options - Search parameters.
   * @returns List of entry objects matching the search, in relevance order.
   */
  async query(options: QueryOptions): Promise<Entry[]> {
    const { text, type, session, tag, file, since, until, limit = 20 } = options;

    const result = await this._send("search", { query: text });
    let entries: Entry[] = Array.isArray(result) ? result : [];

    entries = AgentlogClient._filterEntries(entries, {
      type,
      session,
      tag,
      file,
      since,
      until,
    });

    return entries.slice(0, limit);
  }

  /**
   * List entries with filters.
   *
   * At least one filter should be provided, or the method defaults to
   * entries from the last 24 hours.
   *
   * @param options - Filter parameters.
   * @returns List of entry objects matching the filters, sorted by timestamp.
   */
  async log(options: LogOptions = {}): Promise<Entry[]> {
    const {
      type,
      session,
      tag,
      file,
      since,
      until,
      limit = 50,
      offset = 0,
    } = options;

    const hasFilter = !!(type || session || tag || file || since || until);

    const params: Record<string, unknown> = {};
    let remainingFilters: {
      type?: string;
      session?: string;
      tag?: string;
      file?: string;
      since?: string;
      until?: string;
    } = {};

    if (session) {
      params.session_id = session;
      remainingFilters = { type, tag, file, since, until };
    } else if (type) {
      params.type = type;
      remainingFilters = { session, tag, file, since, until };
    } else if (tag) {
      params.tags = [tag];
      remainingFilters = { type, session, file, since, until };
    } else if (file) {
      params.file_path = file;
      remainingFilters = { type, session, tag, since, until };
    } else if (since || until) {
      params.start = since ? resolveTime(since) : "1970-01-01T00:00:00Z";
      params.end = until ? resolveTime(until) : new Date().toISOString();
      remainingFilters = { type, session, tag, file };
    } else if (!hasFilter) {
      // No filters - default to last 24 hours.
      const now = new Date();
      const start = new Date(now.getTime() - 86_400_000);
      params.start = start.toISOString();
      params.end = now.toISOString();
    }

    const result = await this._send("query", params);
    let entries: Entry[] = Array.isArray(result) ? result : [];

    // Apply any remaining client-side filters.
    entries = AgentlogClient._filterEntries(entries, remainingFilters);

    // Apply offset and limit.
    return entries.slice(offset, offset + limit);
  }

  /**
   * Return a structured context string suitable for prompt injection.
   *
   * Calls the daemon's `context` method to fetch entries by file paths
   * and/or topic, then formats them as a readable text block.
   *
   * At least one of `files` or `topic` must be provided.
   *
   * @param options - Context retrieval parameters.
   * @returns A formatted string with entry summaries.
   */
  async context(options: ContextOptions = {}): Promise<string> {
    const { files, topic, limit } = options;

    const params: Record<string, unknown> = {};
    if (files != null && files.length > 0) {
      params.files = files;
    }
    if (topic != null) {
      params.topic = topic;
    }
    if (limit != null) {
      params.limit = limit;
    }

    const result = await this._send("context", params);
    const entries: Entry[] = Array.isArray(result) ? result : [];

    return AgentlogClient._formatContext(entries);
  }

  /**
   * Export entries as a formatted string.
   *
   * Calls the daemon's `export` protocol method. The daemon handles
   * filtering, sorting, and formatting; this method resolves duration
   * strings client-side and passes all other parameters through.
   *
   * @param options - Export parameters.
   * @returns Formatted string from the daemon.
   */
  async export(options: ExportOptions = {}): Promise<string> {
    const { session, since, until, file, tag, type, format, template } =
      options;

    const params: Record<string, unknown> = {};
    if (session != null) {
      params.session_id = session;
    }
    if (since != null) {
      params.since = resolveTime(since);
    }
    if (until != null) {
      params.until = resolveTime(until);
    }
    if (file != null) {
      params.file_path = file;
    }
    if (tag != null) {
      params.tag = tag;
    }
    if (type != null) {
      params.type = type;
    }
    if (format != null) {
      params.format = format;
    }
    if (template != null) {
      params.template = template;
    }

    const result = await this._send("export", params);
    return typeof result === "string" ? result : "";
  }

  // -- internal helpers -----------------------------------------------------

  /**
   * Apply client-side filters to a list of entry objects.
   */
  static _filterEntries(
    entries: Entry[],
    filters: {
      type?: string;
      session?: string;
      tag?: string;
      file?: string;
      since?: string;
      until?: string;
    }
  ): Entry[] {
    let filtered = entries;

    if (filters.type) {
      filtered = filtered.filter((e) => e.type === filters.type);
    }
    if (filters.session) {
      filtered = filtered.filter((e) => e.session_id === filters.session);
    }
    if (filters.tag) {
      const tag = filters.tag;
      filtered = filtered.filter((e) => (e.tags ?? []).includes(tag));
    }
    if (filters.file) {
      const file = filters.file;
      filtered = filtered.filter((e) => (e.file_refs ?? []).includes(file));
    }
    if (filters.since) {
      const sinceTime = resolveTime(filters.since);
      filtered = filtered.filter((e) => (e.timestamp ?? "") >= sinceTime);
    }
    if (filters.until) {
      const untilTime = resolveTime(filters.until);
      filtered = filtered.filter((e) => (e.timestamp ?? "") <= untilTime);
    }

    return filtered;
  }

  /**
   * Format entries into a structured text block for prompt injection.
   */
  static _formatContext(entries: Entry[]): string {
    if (entries.length === 0) {
      return "# Recent decisions\n\nNo entries found.";
    }

    const lines: string[] = ["# Recent decisions", ""];

    for (const entry of entries) {
      const entryType = entry.type ?? "unknown";
      const title = entry.title ?? "Untitled";
      const timestamp = entry.timestamp ?? "";
      // Format timestamp for display - truncate to minute precision.
      let displayTime = timestamp;
      if (timestamp.includes("T")) {
        displayTime = timestamp.slice(0, 16).replace("T", " ");
      }

      lines.push(`## [${entryType}] ${title} (${displayTime})`);

      if (entry.body) {
        lines.push(entry.body);
      }

      if (entry.tags && entry.tags.length > 0) {
        lines.push(`Tags: ${entry.tags.join(", ")}`);
      }

      if (entry.file_refs && entry.file_refs.length > 0) {
        lines.push(`Files: ${entry.file_refs.join(", ")}`);
      }

      lines.push("");
    }

    return lines.join("\n");
  }
}
