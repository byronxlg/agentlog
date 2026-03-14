---
description: Code quality and dependency management conventions
---

# Code conventions

- Use project-configured linters, run before committing
- Import ordering: stdlib, third-party, local - alphabetical within groups
- Fail fast with meaningful error messages, no silent failures
- Structured logging, no secrets in logs, redact sensitive data

# Dependencies

- Justify new dependencies in the PR description, prefer well-maintained packages
- Update dependencies in dedicated PRs, not mixed with feature work
