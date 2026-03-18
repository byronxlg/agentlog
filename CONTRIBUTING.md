# Contributing to agentlog

Thanks for your interest in contributing to agentlog. This guide covers how to set up the project, run tests, and submit changes.

## Development setup

### Prerequisites

- [Go 1.24+](https://go.dev/dl/)
- [Make](https://www.gnu.org/software/make/)
- [golangci-lint v2](https://golangci-lint.run/welcome/install/)

### Build

```bash
git clone https://github.com/byronxlg/agentlog.git
cd agentlog
make build
```

This produces two binaries in `bin/`: `agentlog` (CLI) and `agentlogd` (daemon).

### Test

```bash
make test
```

### Lint

```bash
make lint
```

Linting rules are defined in `.golangci.yml`.

## Code standards

- **Import ordering:** stdlib, third-party, local - alphabetical within each group
- **Commit messages:** use [conventional commits](https://www.conventionalcommits.org/) - `feat:`, `fix:`, `docs:`, `chore:`, etc.
- **Error handling:** fail fast with meaningful error messages, no silent failures
- **Logging:** use structured logging, never log secrets

## Pull request process

1. **Branch naming:** `issue-{number}-{short-slug}` (e.g., `issue-42-add-auth`)
2. **Reference the issue:** include `Closes #NUMBER` in the PR description
3. **CI must pass:** all PRs run build, test, and lint checks via GitHub Actions
4. **Review required:** all PRs require at least one approval before merge
5. **Squash merge:** PRs are squash-merged to main

Keep PRs focused on a single issue. If you discover unrelated problems during implementation, open a separate issue.

## Issue triage

New issues are reviewed and labeled by maintainers. Here is what to expect:

- **Response time:** we aim to triage new issues within a few days, but timelines vary
- **Labels:** issues are labeled by type (bug, feature, etc.) and priority
- **Ready label:** issues marked `ready` have acceptance criteria defined and are available for contributors to pick up

If your issue is a question or needs discussion, consider using [GitHub Discussions](https://github.com/byronxlg/agentlog/discussions) instead.

### Security vulnerabilities

Do not report security vulnerabilities through public issues. Instead, email the maintainers directly or use [GitHub's private vulnerability reporting](https://github.com/byronxlg/agentlog/security/advisories/new).

## SDK development

### Python SDK

Requires Python 3.9+.

```bash
cd sdk/python
python -m venv .venv
source .venv/bin/activate
pip install -e .
pytest
```

### TypeScript SDK

Requires Node 18+.

```bash
cd sdk/typescript
npm install
npm test
```

Tests use [vitest](https://vitest.dev/).
