---
description: Infrastructure, deployment, and security conventions
---

# Infrastructure

- Deploy via IaC only - never deploy directly via CLI
- Environments: dev, staging, production
- Never commit secrets, use environment variables or a secrets manager
- All PRs must pass CI before merge

# Security

- Validate all external input, sanitize output
- Address critical/high vulnerability findings before merge
