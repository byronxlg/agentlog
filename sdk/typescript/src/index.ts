/**
 * agentlog-sdk - TypeScript SDK for the agentlog decision log daemon.
 *
 * Usage:
 *
 *   import { AgentlogClient } from "agentlog-sdk";
 *
 *   const client = new AgentlogClient();
 *   await client.write({ type: "decision", title: "Use PostgreSQL" });
 *   const entries = await client.query({ text: "database" });
 */

export { AgentlogClient } from "./client.js";
export { AgentlogError, ConnectionError, DaemonNotRunningError } from "./errors.js";
export {
  VALID_ENTRY_TYPES,
  type EntryType,
  type WriteOptions,
  type QueryOptions,
  type LogOptions,
  type ContextOptions,
  type ExportOptions,
  type ClientOptions,
  type Entry,
} from "./types.js";
