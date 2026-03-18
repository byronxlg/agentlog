/**
 * Unit tests for AgentlogClient.
 *
 * Tests cover serialization, error handling, session management, and type
 * validation. A mock Unix socket server is used so no running daemon is required.
 */

import { createServer, Server, Socket } from "node:net";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { describe, it, expect, beforeEach, afterEach } from "vitest";

import { AgentlogClient } from "../src/client.js";
import { parseDuration, resolveTime } from "../src/client.js";
import { AgentlogError, ConnectionError, DaemonNotRunningError } from "../src/errors.js";
import { VALID_ENTRY_TYPES } from "../src/types.js";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

interface DaemonRequest {
  method: string;
  params?: Record<string, unknown>;
}

function makeOkResponse(result: unknown): string {
  return JSON.stringify({ ok: true, result }) + "\n";
}

function makeErrorResponse(msg: string): string {
  return JSON.stringify({ ok: false, error: msg }) + "\n";
}

/**
 * A minimal Unix socket server that records requests and replies with
 * canned responses.
 */
class FakeDaemon {
  readonly socketPath: string;
  readonly requests: DaemonRequest[] = [];

  private _response: unknown = null;
  private _errorMsg: string | null = null;
  private _responseQueue: unknown[] | null = null;
  private _server: Server | null = null;

  constructor(socketPath: string) {
    this.socketPath = socketPath;
  }

  get lastRequest(): DaemonRequest | undefined {
    return this.requests[this.requests.length - 1];
  }

  setResponse(result: unknown): void {
    this._response = result;
    this._errorMsg = null;
    this._responseQueue = null;
  }

  setResponses(results: unknown[]): void {
    this._responseQueue = [...results];
    this._response = null;
    this._errorMsg = null;
  }

  setError(msg: string): void {
    this._errorMsg = msg;
    this._response = null;
    this._responseQueue = null;
  }

  private _nextResponseString(): string {
    if (this._errorMsg != null) {
      return makeErrorResponse(this._errorMsg);
    }
    if (this._responseQueue != null && this._responseQueue.length > 0) {
      return makeOkResponse(this._responseQueue.shift());
    }
    return makeOkResponse(this._response);
  }

  async start(): Promise<void> {
    return new Promise<void>((resolve) => {
      this._server = createServer((conn: Socket) => {
        let buf = "";
        conn.on("data", (chunk: Buffer) => {
          buf += chunk.toString("utf-8");
          if (buf.includes("\n")) {
            const req = JSON.parse(buf.trim()) as DaemonRequest;
            this.requests.push(req);
            conn.write(this._nextResponseString());
            conn.end();
          }
        });
      });
      this._server.listen(this.socketPath, () => {
        resolve();
      });
    });
  }

  async stop(): Promise<void> {
    return new Promise<void>((resolve) => {
      if (this._server) {
        this._server.close(() => resolve());
      } else {
        resolve();
      }
    });
  }
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

let sockDir: string;
let sockPath: string;

beforeEach(() => {
  sockDir = mkdtempSync(join(tmpdir(), "al_"));
  sockPath = join(sockDir, "t.sock");
});

afterEach(() => {
  rmSync(sockDir, { recursive: true, force: true });
});

// ---------------------------------------------------------------------------
// Type validation tests
// ---------------------------------------------------------------------------

describe("type validation", () => {
  it("accepts all valid types", async () => {
    const daemon = new FakeDaemon(sockPath);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      // Bypass auto-session by setting the internal property.
      (client as unknown as { _sessionId: string })._sessionId = "sess-1";

      for (const entryType of [...VALID_ENTRY_TYPES].sort()) {
        daemon.setResponse({
          id: `entry-${entryType}`,
          timestamp: "2026-03-15T10:00:00Z",
          session_id: "sess-1",
          type: entryType,
          title: "Test",
        });
        const entryId = await client.write({
          type: entryType,
          title: "Test",
        });
        expect(entryId).toBe(`entry-${entryType}`);
      }
    } finally {
      await daemon.stop();
    }
  });

  it("rejects invalid type with an error", async () => {
    const client = new AgentlogClient({ socketPath: "/nonexistent.sock" });
    await expect(
      client.write({
        type: "invalid_type" as never,
        title: "Test",
      })
    ).rejects.toThrow("invalid entry type");
  });

  it("has the correct set of valid entry types", () => {
    expect([...VALID_ENTRY_TYPES].sort()).toEqual([
      "assumption",
      "attempt_failed",
      "decision",
      "deferred",
      "question",
    ]);
  });
});

// ---------------------------------------------------------------------------
// Serialization tests
// ---------------------------------------------------------------------------

describe("serialization", () => {
  it("serializes write request with all fields", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse({
      id: "entry-1",
      timestamp: "2026-03-15T10:00:00Z",
      session_id: "sess-1",
      type: "decision",
      title: "Use Redis",
      body: "For caching",
      tags: ["infrastructure"],
      file_refs: ["config.yaml"],
    });
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      (client as unknown as { _sessionId: string })._sessionId = "sess-1";

      await client.write({
        type: "decision",
        title: "Use Redis",
        body: "For caching",
        tags: ["infrastructure"],
        files: ["config.yaml"],
      });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("write");
      const entry = (req.params as Record<string, unknown>)
        .entry as Record<string, unknown>;
      expect(entry.type).toBe("decision");
      expect(entry.title).toBe("Use Redis");
      expect(entry.body).toBe("For caching");
      expect(entry.tags).toEqual(["infrastructure"]);
      expect(entry.file_refs).toEqual(["config.yaml"]);
      expect(entry.session_id).toBe("sess-1");
    } finally {
      await daemon.stop();
    }
  });

  it("omits optional fields when not provided", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse({
      id: "entry-1",
      timestamp: "2026-03-15T10:00:00Z",
      session_id: "sess-1",
      type: "decision",
      title: "Minimal entry",
    });
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      (client as unknown as { _sessionId: string })._sessionId = "sess-1";

      await client.write({ type: "decision", title: "Minimal entry" });

      const req = daemon.lastRequest!;
      const entry = (req.params as Record<string, unknown>)
        .entry as Record<string, unknown>;
      expect(entry).not.toHaveProperty("body");
      expect(entry).not.toHaveProperty("tags");
      expect(entry).not.toHaveProperty("file_refs");
    } finally {
      await daemon.stop();
    }
  });

  it("serializes search query", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.query({ text: "database migration" });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("search");
      expect((req.params as Record<string, unknown>).query).toBe(
        "database migration"
      );
    } finally {
      await daemon.stop();
    }
  });

  it("serializes log filter by session", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.log({ session: "sess-1" });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("query");
      expect((req.params as Record<string, unknown>).session_id).toBe("sess-1");
    } finally {
      await daemon.stop();
    }
  });

  it("serializes log filter by type", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.log({ type: "decision" });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("query");
      expect((req.params as Record<string, unknown>).type).toBe("decision");
    } finally {
      await daemon.stop();
    }
  });

  it("serializes log filter by tag", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.log({ tag: "infrastructure" });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("query");
      expect((req.params as Record<string, unknown>).tags).toEqual([
        "infrastructure",
      ]);
    } finally {
      await daemon.stop();
    }
  });

  it("serializes log filter by file", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.log({ file: "main.go" });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("query");
      expect((req.params as Record<string, unknown>).file_path).toBe("main.go");
    } finally {
      await daemon.stop();
    }
  });
});

// ---------------------------------------------------------------------------
// Session management tests
// ---------------------------------------------------------------------------

describe("session management", () => {
  it("auto-creates session on first write", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponses([
      // First request: create_session
      { session_id: "auto-sess-1" },
      // Second request: write
      {
        id: "entry-1",
        timestamp: "2026-03-15T10:00:00Z",
        session_id: "auto-sess-1",
        type: "decision",
        title: "Test",
      },
    ]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      expect(client.sessionId).toBeNull();

      const entryId = await client.write({ type: "decision", title: "Test" });

      expect(client.sessionId).toBe("auto-sess-1");
      expect(entryId).toBe("entry-1");
      expect(daemon.requests).toHaveLength(2);
      expect(daemon.requests[0].method).toBe("create_session");
      expect(daemon.requests[1].method).toBe("write");
    } finally {
      await daemon.stop();
    }
  });

  it("reuses session across writes", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse({
      id: "entry-1",
      timestamp: "2026-03-15T10:00:00Z",
      session_id: "existing-sess",
      type: "decision",
      title: "Test",
    });
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      (client as unknown as { _sessionId: string })._sessionId = "existing-sess";

      await client.write({ type: "decision", title: "First" });
      await client.write({ type: "decision", title: "Second" });

      // Both writes should use the same session - no create_session calls.
      expect(daemon.requests.every((r) => r.method === "write")).toBe(true);
      expect(
        daemon.requests.every(
          (r) =>
            (
              (r.params as Record<string, unknown>).entry as Record<
                string,
                unknown
              >
            ).session_id === "existing-sess"
        )
      ).toBe(true);
    } finally {
      await daemon.stop();
    }
  });

  it("explicit session overrides auto session", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse({
      id: "entry-1",
      timestamp: "2026-03-15T10:00:00Z",
      session_id: "explicit-sess",
      type: "decision",
      title: "Test",
    });
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      (client as unknown as { _sessionId: string })._sessionId = "auto-sess";

      await client.write({
        type: "decision",
        title: "Test",
        session: "explicit-sess",
      });

      const req = daemon.lastRequest!;
      const entry = (req.params as Record<string, unknown>)
        .entry as Record<string, unknown>;
      expect(entry.session_id).toBe("explicit-sess");
      // Auto session should still be the original.
      expect(client.sessionId).toBe("auto-sess");
    } finally {
      await daemon.stop();
    }
  });
});

// ---------------------------------------------------------------------------
// Error handling tests
// ---------------------------------------------------------------------------

describe("error handling", () => {
  it("throws DaemonNotRunningError when socket does not exist", async () => {
    const client = new AgentlogClient({
      socketPath: "/nonexistent/path/test.sock",
    });
    await expect(client._send("create_session")).rejects.toThrow(
      DaemonNotRunningError
    );
    await expect(client._send("create_session")).rejects.toThrow(
      "daemon socket not found"
    );
  });

  it("throws AgentlogError on daemon error response", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setError("something went wrong");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await expect(client._send("bad_method")).rejects.toThrow(AgentlogError);
      await expect(client._send("bad_method")).rejects.toThrow(
        "something went wrong"
      );
    } finally {
      await daemon.stop();
    }
  });

  it("throws ConnectionError when socket file exists but is not a socket", async () => {
    // Create a regular file (not a socket) so existsSync returns true
    // but net.connect fails.
    const brokenPath = join(sockDir, "broken.sock");
    writeFileSync(brokenPath, "");

    const client = new AgentlogClient({ socketPath: brokenPath });
    await expect(client._send("create_session")).rejects.toThrow(
      ConnectionError
    );
    await expect(client._send("create_session")).rejects.toThrow(
      "failed to connect"
    );
  });
});

// ---------------------------------------------------------------------------
// Duration parsing tests
// ---------------------------------------------------------------------------

describe("duration parsing", () => {
  it("parses seconds", () => {
    expect(parseDuration("30s")).toBe(30_000);
  });

  it("parses minutes", () => {
    expect(parseDuration("5m")).toBe(300_000);
  });

  it("parses hours", () => {
    expect(parseDuration("2h")).toBe(7_200_000);
  });

  it("parses days", () => {
    expect(parseDuration("7d")).toBe(604_800_000);
  });

  it("parses weeks", () => {
    expect(parseDuration("1w")).toBe(604_800_000);
  });

  it("throws on invalid suffix", () => {
    expect(() => parseDuration("5x")).toThrow("unsupported duration suffix");
  });

  it("throws on empty string", () => {
    expect(() => parseDuration("")).toThrow("empty duration");
  });

  it("passes through ISO 8601 strings", () => {
    const iso = "2026-03-15T10:30:00Z";
    expect(resolveTime(iso)).toBe(iso);
  });

  it("resolves relative durations", () => {
    const result = resolveTime("1h");
    // Should be a valid ISO string.
    expect(result).toContain("T");
    expect(result).toMatch(/Z$/);
  });
});

// ---------------------------------------------------------------------------
// Client configuration tests
// ---------------------------------------------------------------------------

describe("client configuration", () => {
  it("uses default socket path", () => {
    const client = new AgentlogClient();
    expect(client.socketPath).toMatch(/\.agentlog\/agentlogd\.sock$/);
  });

  it("uses custom agentlogDir", () => {
    const client = new AgentlogClient({ agentlogDir: "/custom/dir" });
    expect(client.socketPath).toBe("/custom/dir/agentlogd.sock");
  });

  it("socketPath overrides agentlogDir", () => {
    const client = new AgentlogClient({
      agentlogDir: "/custom/dir",
      socketPath: "/other/path.sock",
    });
    expect(client.socketPath).toBe("/other/path.sock");
  });

  it("reads AGENTLOG_DIR from environment", () => {
    const orig = process.env.AGENTLOG_DIR;
    try {
      process.env.AGENTLOG_DIR = "/env/dir";
      const client = new AgentlogClient();
      expect(client.socketPath).toBe("/env/dir/agentlogd.sock");
    } finally {
      if (orig != null) {
        process.env.AGENTLOG_DIR = orig;
      } else {
        delete process.env.AGENTLOG_DIR;
      }
    }
  });

  it("explicit agentlogDir overrides env", () => {
    const orig = process.env.AGENTLOG_DIR;
    try {
      process.env.AGENTLOG_DIR = "/env/dir";
      const client = new AgentlogClient({ agentlogDir: "/explicit/dir" });
      expect(client.socketPath).toBe("/explicit/dir/agentlogd.sock");
    } finally {
      if (orig != null) {
        process.env.AGENTLOG_DIR = orig;
      } else {
        delete process.env.AGENTLOG_DIR;
      }
    }
  });
});

// ---------------------------------------------------------------------------
// Context formatting tests
// ---------------------------------------------------------------------------

describe("context formatting", () => {
  it("formats empty entries", () => {
    const result = AgentlogClient._formatContext([]);
    expect(result).toContain("No entries found");
  });

  it("formats a single entry with all fields", () => {
    const entries = [
      {
        id: "entry-1",
        timestamp: "2026-03-15T10:30:00Z",
        session_id: "sess-1",
        type: "decision",
        title: "Use PostgreSQL",
        body: "Better for relational data.",
        tags: ["database", "infrastructure"],
        file_refs: ["config.yaml", "docker-compose.yml"],
      },
    ];
    const result = AgentlogClient._formatContext(entries);
    expect(result).toContain("# Recent decisions");
    expect(result).toContain(
      "## [decision] Use PostgreSQL (2026-03-15 10:30)"
    );
    expect(result).toContain("Better for relational data.");
    expect(result).toContain("Tags: database, infrastructure");
    expect(result).toContain("Files: config.yaml, docker-compose.yml");
  });

  it("formats entry without optional fields", () => {
    const entries = [
      {
        id: "entry-1",
        timestamp: "2026-03-15T10:30:00Z",
        session_id: "sess-1",
        type: "assumption",
        title: "Users have Node 18+",
      },
    ];
    const result = AgentlogClient._formatContext(entries);
    expect(result).toContain("## [assumption] Users have Node 18+");
    expect(result).not.toContain("Tags:");
    expect(result).not.toContain("Files:");
  });
});

// ---------------------------------------------------------------------------
// Client-side filtering tests
// ---------------------------------------------------------------------------

describe("client-side filtering", () => {
  it("filters by type", () => {
    const entries = [
      { id: "1", timestamp: "", session_id: "", type: "decision", title: "A" },
      {
        id: "2",
        timestamp: "",
        session_id: "",
        type: "assumption",
        title: "B",
      },
      { id: "3", timestamp: "", session_id: "", type: "decision", title: "C" },
    ];
    const result = AgentlogClient._filterEntries(entries, {
      type: "decision",
    });
    expect(result).toHaveLength(2);
    expect(result.every((e) => e.type === "decision")).toBe(true);
  });

  it("filters by tag", () => {
    const entries = [
      {
        id: "1",
        timestamp: "",
        session_id: "",
        type: "decision",
        title: "A",
        tags: ["db", "infra"],
      },
      {
        id: "2",
        timestamp: "",
        session_id: "",
        type: "decision",
        title: "B",
        tags: ["api"],
      },
      { id: "3", timestamp: "", session_id: "", type: "decision", title: "C" },
    ];
    const result = AgentlogClient._filterEntries(entries, { tag: "db" });
    expect(result).toHaveLength(1);
    expect(result[0].title).toBe("A");
  });

  it("filters by file", () => {
    const entries = [
      {
        id: "1",
        timestamp: "",
        session_id: "",
        type: "decision",
        title: "A",
        file_refs: ["main.go"],
      },
      {
        id: "2",
        timestamp: "",
        session_id: "",
        type: "decision",
        title: "B",
        file_refs: ["config.yaml"],
      },
      { id: "3", timestamp: "", session_id: "", type: "decision", title: "C" },
    ];
    const result = AgentlogClient._filterEntries(entries, { file: "main.go" });
    expect(result).toHaveLength(1);
    expect(result[0].title).toBe("A");
  });

  it("filters by session", () => {
    const entries = [
      { id: "1", timestamp: "", session_id: "s1", type: "decision", title: "A" },
      { id: "2", timestamp: "", session_id: "s2", type: "decision", title: "B" },
    ];
    const result = AgentlogClient._filterEntries(entries, { session: "s1" });
    expect(result).toHaveLength(1);
    expect(result[0].title).toBe("A");
  });

  it("applies limit in query", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse(
      Array.from({ length: 10 }, (_, i) => ({
        id: `e${i}`,
        timestamp: "",
        session_id: "",
        type: "decision",
        title: `Entry ${i}`,
      }))
    );
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const results = await client.query({ text: "test", limit: 3 });
      expect(results).toHaveLength(3);
    } finally {
      await daemon.stop();
    }
  });
});

// ---------------------------------------------------------------------------
// Log method tests
// ---------------------------------------------------------------------------

describe("log method", () => {
  it("defaults to time range when no filters are provided", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.log();

      const req = daemon.lastRequest!;
      expect(req.method).toBe("query");
      expect(req.params).toHaveProperty("start");
      expect(req.params).toHaveProperty("end");
    } finally {
      await daemon.stop();
    }
  });

  it("applies offset correctly", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse(
      Array.from({ length: 10 }, (_, i) => ({
        id: `e${i}`,
        timestamp: "",
        session_id: "",
        type: "decision",
        title: `Entry ${i}`,
      }))
    );
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const results = await client.log({
        type: "decision",
        limit: 3,
        offset: 2,
      });
      expect(results).toHaveLength(3);
      expect(results[0].title).toBe("Entry 2");
    } finally {
      await daemon.stop();
    }
  });

  it("handles since duration filter", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.log({ since: "1h" });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("query");
      expect(req.params).toHaveProperty("start");
      expect(req.params).toHaveProperty("end");
    } finally {
      await daemon.stop();
    }
  });
});

// ---------------------------------------------------------------------------
// Context method tests
// ---------------------------------------------------------------------------

describe("context method", () => {
  it("calls daemon context method with files", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([
      {
        id: "e1",
        timestamp: "2026-03-15T10:00:00Z",
        session_id: "s1",
        type: "decision",
        title: "Redis decision",
        file_refs: ["config/redis.yaml"],
      },
    ]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.context({ files: ["config/redis.yaml"] });

      expect(daemon.lastRequest!.method).toBe("context");
      expect(
        (daemon.lastRequest!.params as Record<string, unknown>).files
      ).toEqual(["config/redis.yaml"]);
      expect(result).toContain("Redis decision");
    } finally {
      await daemon.stop();
    }
  });

  it("calls daemon context method with topic", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([
      {
        id: "e1",
        timestamp: "2026-03-15T10:00:00Z",
        session_id: "s1",
        type: "decision",
        title: "Auth decision",
      },
    ]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.context({ topic: "authentication" });

      expect(daemon.lastRequest!.method).toBe("context");
      expect(
        (daemon.lastRequest!.params as Record<string, unknown>).topic
      ).toBe("authentication");
      expect(result).toContain("Auth decision");
    } finally {
      await daemon.stop();
    }
  });

  it("passes both files and topic to daemon", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([
      {
        id: "e1",
        timestamp: "2026-03-15T10:00:00Z",
        session_id: "s1",
        type: "decision",
        title: "Combined result",
      },
    ]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.context({
        files: ["main.go"],
        topic: "caching",
      });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(daemon.lastRequest!.method).toBe("context");
      expect(params.files).toEqual(["main.go"]);
      expect(params.topic).toBe("caching");
      expect(result).toContain("Combined result");
    } finally {
      await daemon.stop();
    }
  });

  it("passes limit to daemon", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.context({ topic: "test", limit: 5 });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.limit).toBe(5);
    } finally {
      await daemon.stop();
    }
  });

  it("returns formatted context string", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse([]);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.context({ topic: "nonexistent" });

      expect(result).toContain("No entries found");
    } finally {
      await daemon.stop();
    }
  });
});

// ---------------------------------------------------------------------------
// Export method tests
// ---------------------------------------------------------------------------

describe("export method", () => {
  it("calls daemon export method and returns result", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("# Decision Log Export\n\n## Use Redis\n");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.export();

      expect(daemon.lastRequest!.method).toBe("export");
      expect(result).toBe("# Decision Log Export\n\n## Use Redis\n");
    } finally {
      await daemon.stop();
    }
  });

  it("passes session parameter", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ session: "sess-123" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.session_id).toBe("sess-123");
    } finally {
      await daemon.stop();
    }
  });

  it("resolves since duration to ISO 8601", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ since: "7d" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      const since = params.since as string;
      expect(since).toContain("T");
      expect(since).toMatch(/Z$/);
    } finally {
      await daemon.stop();
    }
  });

  it("resolves until duration to ISO 8601", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ until: "1h" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      const until = params.until as string;
      expect(until).toContain("T");
      expect(until).toMatch(/Z$/);
    } finally {
      await daemon.stop();
    }
  });

  it("passes through ISO 8601 since value", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ since: "2026-03-01T00:00:00Z" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.since).toBe("2026-03-01T00:00:00Z");
    } finally {
      await daemon.stop();
    }
  });

  it("passes file parameter", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ file: "main.go" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.file_path).toBe("main.go");
    } finally {
      await daemon.stop();
    }
  });

  it("passes tag parameter", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ tag: "infrastructure" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.tag).toBe("infrastructure");
    } finally {
      await daemon.stop();
    }
  });

  it("passes type parameter", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ type: "decision" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.type).toBe("decision");
    } finally {
      await daemon.stop();
    }
  });

  it("passes format json", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("[]");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.export({ format: "json" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.format).toBe("json");
      expect(result).toBe("[]");
    } finally {
      await daemon.stop();
    }
  });

  it("passes format text", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("No entries found.");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.export({ format: "text" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.format).toBe("text");
      expect(result).toBe("No entries found.");
    } finally {
      await daemon.stop();
    }
  });

  it("passes format markdown", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("# Decision Log Export\n");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ format: "markdown" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.format).toBe("markdown");
    } finally {
      await daemon.stop();
    }
  });

  it("passes template pr", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("## What changed\n\n- **Use Redis**\n");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.export({ template: "pr" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.template).toBe("pr");
      expect(result).toContain("What changed");
    } finally {
      await daemon.stop();
    }
  });

  it("passes template retro", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("# Retrospective\n");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ template: "retro" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.template).toBe("retro");
    } finally {
      await daemon.stop();
    }
  });

  it("passes template handoff", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("# Handoff Document\n");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ template: "handoff" });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.template).toBe("handoff");
    } finally {
      await daemon.stop();
    }
  });

  it("handles empty result", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("No entries found.");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.export();

      expect(result).toBe("No entries found.");
    } finally {
      await daemon.stop();
    }
  });

  it("omits undefined parameters", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({ tag: "db" });

      const req = daemon.lastRequest!;
      expect(req.method).toBe("export");
      expect(req.params).toEqual({ tag: "db" });
    } finally {
      await daemon.stop();
    }
  });

  it("passes all parameters combined", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse("exported");
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      await client.export({
        session: "sess-1",
        since: "2026-03-01T00:00:00Z",
        until: "2026-03-15T00:00:00Z",
        file: "main.go",
        tag: "db",
        type: "decision",
        format: "text",
        template: "pr",
      });

      const params = daemon.lastRequest!.params as Record<string, unknown>;
      expect(params.session_id).toBe("sess-1");
      expect(params.since).toBe("2026-03-01T00:00:00Z");
      expect(params.until).toBe("2026-03-15T00:00:00Z");
      expect(params.file_path).toBe("main.go");
      expect(params.tag).toBe("db");
      expect(params.type).toBe("decision");
      expect(params.format).toBe("text");
      expect(params.template).toBe("pr");
    } finally {
      await daemon.stop();
    }
  });

  it("returns empty string for non-string result", async () => {
    const daemon = new FakeDaemon(sockPath);
    daemon.setResponse(42);
    await daemon.start();

    try {
      const client = new AgentlogClient({ socketPath: sockPath });
      const result = await client.export();

      expect(result).toBe("");
    } finally {
      await daemon.stop();
    }
  });
});
