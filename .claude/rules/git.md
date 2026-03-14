---
description: Git workflow conventions for branches, commits, and merging
---

# Git conventions

- Branch naming: `issue-{number}-{short-slug}` (e.g., `issue-42-add-auth`)
- Commit message format: conventional commits (feat:, fix:, docs:, chore:) referencing the issue
- Merge strategy: squash merge to main
- Protected branches: main - no direct commits, PRs only
