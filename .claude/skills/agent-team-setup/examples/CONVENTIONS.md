# Project Conventions

Project-specific development conventions. Once complete, give it to Claude Code
with the prompt: "Use CONVENTIONS.md to generate the project rules."

These become `.claude/rules/*.md` files - auto-loaded into every conversation.
Only include what applies to your project. Delete any section that doesn't.


---

## Git

- Branch naming: `issue-{number}-{short-slug}` (e.g., `issue-42-add-auth`)
- Commit message format: conventional commits (feat:, fix:, docs:, chore:) referencing the issue
- Merge strategy: squash merge to main
- Protected branches: main - no direct commits, PRs only


---

## Code

- Linting/formatting tools: use project-configured linters, run before committing
- Import ordering: stdlib, third-party, local - alphabetical within groups
- Error handling approach: fail fast, meaningful error messages, no silent failures
- Logging conventions: structured logging, no secrets in logs, redact sensitive data


---

## Testing

- What requires tests: all new functionality and bug fixes
- Test naming convention: describe what is being tested and the expected outcome
- Post-deployment testing: smoke test core paths after every deployment


---

## Infrastructure

- Deployment method: IaC only - never deploy directly via CLI
- Environments: dev, staging, production
- Secrets management: never commit secrets, use environment variables or a secrets manager
- CI/CD pipeline: all PRs must pass CI before merge


---

## Dependencies

- Approval process for new dependencies: justify in the PR description, prefer well-maintained packages
- Update strategy: keep dependencies current, update in dedicated PRs not mixed with feature work


---

## Documentation

- Where docs live: in the repo alongside the code
- What requires documentation: public APIs, architecture decisions, non-obvious design choices
---

## Security

- Data handling requirements: validate all external input, sanitize output
- Vulnerability scanning: address critical/high findings before merge
