/**
 * Integration tests for the agentlog TypeScript SDK.
 *
 * These tests require a running agentlog daemon. They are automatically skipped
 * if the daemon socket does not exist.
 */

import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { describe, it, expect, beforeAll } from "vitest";

import { AgentlogClient } from "../src/client.js";

// Determine socket path from env or default.
const agentlogDir = process.env.AGENTLOG_DIR ?? join(homedir(), ".agentlog");
const socketPath = join(agentlogDir, "agentlogd.sock");
const daemonRunning = existsSync(socketPath);

// Helper to skip all tests if daemon is not running.
const describeIfDaemon = daemonRunning ? describe : describe.skip;

describeIfDaemon("integration: write", () => {
  it("writes an entry and queries for it", async () => {
    const client = new AgentlogClient();

    const entryId = await client.write({
      type: "decision",
      title: "Integration test: use SQLite for local storage",
      body: "SQLite is lightweight and requires no separate process.",
      tags: ["integration-test", "database"],
      files: ["internal/store/store.go"],
    });

    expect(entryId).toBeTruthy();
    expect(typeof entryId).toBe("string");

    // Search for the entry we just wrote.
    const results = await client.query({ text: "SQLite local storage" });
    expect(results.length).toBeGreaterThan(0);

    const ids = results.map((r) => r.id);
    expect(ids).toContain(entryId);
  });

  it("returns different IDs for different writes", async () => {
    const client = new AgentlogClient();

    const id1 = await client.write({
      type: "assumption",
      title: "Integration test: unique ID 1",
    });
    const id2 = await client.write({
      type: "assumption",
      title: "Integration test: unique ID 2",
    });

    expect(id1).not.toBe(id2);
  });
});

describeIfDaemon("integration: session", () => {
  it("auto-creates session on first write", async () => {
    const client = new AgentlogClient();
    expect(client.sessionId).toBeNull();

    await client.write({
      type: "decision",
      title: "Integration test: auto session",
    });

    expect(client.sessionId).toBeTruthy();
    expect(typeof client.sessionId).toBe("string");
  });

  it("reuses session across writes", async () => {
    const client = new AgentlogClient();

    await client.write({
      type: "decision",
      title: "Integration test: session reuse 1",
    });
    const firstSession = client.sessionId;

    await client.write({
      type: "decision",
      title: "Integration test: session reuse 2",
    });
    expect(client.sessionId).toBe(firstSession);
  });
});

describeIfDaemon("integration: log", () => {
  it("retrieves entries by type", async () => {
    const client = new AgentlogClient();

    await client.write({
      type: "question",
      title: "Integration test: log by type",
    });

    const results = await client.log({ type: "question" });
    expect(results.length).toBeGreaterThan(0);
    expect(results.every((r) => r.type === "question")).toBe(true);
  });

  it("retrieves entries by session", async () => {
    const client = new AgentlogClient();

    await client.write({
      type: "decision",
      title: "Integration test: log by session 1",
    });
    await client.write({
      type: "assumption",
      title: "Integration test: log by session 2",
    });
    const sessionId = client.sessionId!;

    const results = await client.log({ session: sessionId });
    expect(results.length).toBeGreaterThanOrEqual(2);
  });
});

describeIfDaemon("integration: context", () => {
  it("retrieves context by file refs", async () => {
    const client = new AgentlogClient();

    await client.write({
      type: "decision",
      title: "Integration test: context by files",
      body: "Testing file-based context retrieval.",
      files: ["integration/context-test.go"],
    });

    const result = await client.context({
      files: ["integration/context-test.go"],
    });
    expect(result).toContain("# Recent decisions");
    expect(result).toContain("context by files");
  });

  it("retrieves context by topic", async () => {
    const client = new AgentlogClient();

    await client.write({
      type: "decision",
      title: "Integration test: context topic retrieval",
      body: "Testing topic-based context lookup.",
    });

    const result = await client.context({ topic: "context topic retrieval" });
    expect(result).toContain("context topic retrieval");
  });
});
