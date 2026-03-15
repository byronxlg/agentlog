# TypeScript SDK

TypeScript client for agentlog. Provides programmatic access to the decision log daemon from Node.js.

## Installation

```bash
npm install agentlog-sdk
```

## Requirements

- Node.js 18+
- A running `agentlogd` daemon (see [Getting Started](getting-started.md))

## Quickstart

```typescript
import { AgentlogClient } from "agentlog-sdk";

const client = new AgentlogClient();
await client.write({ type: "decision", title: "Use PostgreSQL for persistence" });
const entries = await client.query({ text: "database" });
```

## Usage

### Writing entries

```typescript
import { AgentlogClient } from "agentlog-sdk";

const client = new AgentlogClient();

// Write a decision entry (session created automatically)
await client.write({
  type: "decision",
  title: "Use Redis for caching",
  body: "Redis provides sub-millisecond reads and built-in TTL support.",
  tags: ["infrastructure", "caching"],
  files: ["config/redis.yaml"],
});

// Supported entry types: decision, attempt_failed, deferred, assumption, question
await client.write({ type: "assumption", title: "All users have Node 18+" });
await client.write({ type: "question", title: "Should we use async or sync HTTP client?" });
```

### Searching entries

```typescript
// Full-text search
const results = await client.query({ text: "database migration" });

// Search with filters
const filtered = await client.query({ text: "caching", type: "decision", limit: 5 });
```

### Listing entries

```typescript
// List entries by type
const entries = await client.log({ type: "decision" });

// List entries by session
const sessionEntries = await client.log({ session: "your-session-id" });

// List entries by tag
const tagEntries = await client.log({ tag: "infrastructure" });

// List entries from the last hour
const recentEntries = await client.log({ since: "1h" });
```

### Getting context for prompts

```typescript
// Get a formatted text block for prompt injection
const context = await client.context({ query: "authentication" });
console.log(context);
// Output:
// # Recent decisions
//
// ## [decision] Use JWT for API auth (2026-03-15 10:30)
// JWTs are stateless and work well with our microservices architecture.
// Tags: auth, api
// Files: internal/auth/jwt.go
```

### Configuration

The SDK looks for the daemon socket at `~/.agentlog/agentlogd.sock` by default. Override this with:

- The `AGENTLOG_DIR` environment variable
- The `agentlogDir` constructor option
- The `socketPath` constructor option (takes precedence)

```typescript
// Custom data directory
const client = new AgentlogClient({ agentlogDir: "/custom/path" });

// Explicit socket path
const client2 = new AgentlogClient({ socketPath: "/tmp/agentlogd.sock" });
```

### Error handling

```typescript
import {
  AgentlogClient,
  AgentlogError,
  ConnectionError,
  DaemonNotRunningError,
} from "agentlog-sdk";

const client = new AgentlogClient();

try {
  await client.write({ type: "decision", title: "Test entry" });
} catch (err) {
  if (err instanceof DaemonNotRunningError) {
    console.log("Start the daemon first: agentlog start");
  } else if (err instanceof ConnectionError) {
    console.log(`Connection failed: ${err.message}`);
  } else if (err instanceof AgentlogError) {
    console.log(`Unexpected error: ${err.message}`);
  }
}
```

## Development

```bash
# Install dependencies
npm install

# Build
npx tsc

# Run tests
npx vitest run

# Run only unit tests (no daemon required)
npx vitest run tests/client.test.ts
```
