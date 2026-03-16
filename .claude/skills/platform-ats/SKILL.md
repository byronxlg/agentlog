---
name: platform-ats
description: Platform commands and state mapping for GitHub-based workflow. Referenced by all role skills for concrete gh CLI commands.
---

# Platform: GitHub

All platform operations use the `gh` CLI. Repo: `byronxlg/agentlog`.

## Identity resolution

Agents are stateless and may not have a persistent identity. Determine your GitHub
identity before any identity-dependent queries:

```
gh api user -q .login
```

Use the returned login in place of `YOUR_LOGIN` in commands below. Do not use `@me`
or other identity-dependent shortcuts.

## State-to-platform mapping

| Workflow State | GitHub Projects Column | Label (if any) |
|---------------|----------------------|----------------|
| Backlog | Backlog | - |
| Ready | Ready | `ready` |
| In Progress | In Progress | - |
| Blocked | Blocked | `blocked` |
| In Review | In Review | - |
| Done | Done | - |
| Awaiting Human | Awaiting Human | - |
| Validated | Validated | - |

When changing workflow state, update both the board column and any associated label.

## Board management

Look up project and field IDs (run once, save for reuse):

```
gh project list --owner byronxlg --format json
gh project field-list PROJECT_NUMBER --owner byronxlg --format json
```

List project items (to find ITEM_ID for a given issue):

```
gh project item-list PROJECT_NUMBER --owner byronxlg --format json
```

Move an issue to a board column:

```
gh project item-edit --project-id PROJECT_ID --id ITEM_ID --field-id STATUS_FIELD_ID --single-select-option-id OPTION_ID
```

Note: ITEM_ID is the project item ID, not the issue number.

## Issues

List open issues:

```
gh issue list -R byronxlg/agentlog --state open --json number,title,labels,assignees
```

List by label:

```
gh issue list -R byronxlg/agentlog --label LABEL
```

List unassigned by label:

```
gh issue list -R byronxlg/agentlog --label LABEL --no-assignee
```

List by assignee:

```
gh issue list -R byronxlg/agentlog --assignee LOGIN --state open
```

List recently closed:

```
gh issue list -R byronxlg/agentlog --state closed --limit 20
```

List epics:

```
gh issue list -R byronxlg/agentlog --label epic
```

View an issue:

```
gh issue view ISSUE_NUMBER -R byronxlg/agentlog
```

Create an issue:

```
gh issue create -R byronxlg/agentlog --title "TITLE" --body "BODY" --label "LABELS"
```

Edit issue body:

```
gh issue edit ISSUE_NUMBER -R byronxlg/agentlog --body "UPDATED_BODY"
```

Add/remove labels:

```
gh issue edit ISSUE_NUMBER -R byronxlg/agentlog --add-label LABEL
gh issue edit ISSUE_NUMBER -R byronxlg/agentlog --remove-label LABEL
```

Assign an issue:

```
gh issue edit ISSUE_NUMBER -R byronxlg/agentlog --add-assignee LOGIN
```

Comment on an issue:

```
gh issue comment ISSUE_NUMBER -R byronxlg/agentlog --body "COMMENT"
```

Link sub-issue to epic:

Use the GitHub MCP tool `mcp__github__sub_issue_write`.

## Pull requests

List open PRs:

```
gh pr list -R byronxlg/agentlog
```

List PRs awaiting review:

```
gh pr list -R byronxlg/agentlog --search "review:required"
```

List PRs with changes requested (by author):

```
gh pr list -R byronxlg/agentlog --author LOGIN --search "review:changes_requested"
```

View a PR:

```
gh pr view PR_NUMBER -R byronxlg/agentlog
```

View PR diff:

```
gh pr diff PR_NUMBER -R byronxlg/agentlog
```

Create a PR:

```
gh pr create -R byronxlg/agentlog --title "TITLE" --body "BODY"
```

Squash merge to main. PR must reference the issue (`Closes #NUMBER`).

Merge after approval:

```
gh pr merge PR_NUMBER -R byronxlg/agentlog --squash
```

Check CI status on a PR:

```
gh pr checks PR_NUMBER -R byronxlg/agentlog
```

Submit a review (approve):

```
gh pr review PR_NUMBER -R byronxlg/agentlog --approve --body "COMMENT"
```

Submit a review (request changes):

```
gh pr review PR_NUMBER -R byronxlg/agentlog --request-changes --body "COMMENT"
```

Comment on specific lines:

```
gh api repos/byronxlg/agentlog/pulls/PR_NUMBER/comments -f body="COMMENT" -f path="FILE" -F line=LINE_NUMBER -f commit_id="COMMIT_SHA"
```

## CI

Check recent workflow runs:

```
gh run list -R byronxlg/agentlog --limit 10
```

## Discussion channels

Look up repo ID (run once):

```
gh repo view byronxlg/agentlog --json id -q .id
```

Look up discussion category IDs (run once):

```
gh api graphql -f query='
  query {
    repository(owner: "byronxlg", name: "agentlog") {
      discussionCategories(first: 20) {
        nodes { id name }
      }
    }
  }'
```

List discussions in a category:

```
gh api graphql -f query='
  query {
    repository(owner: "byronxlg", name: "agentlog") {
      discussions(first: 20, categoryId: "CATEGORY_ID") {
        nodes { id number title body
          comments(first: 10) { nodes { body author { login } } }
        }
      }
    }
  }'
```

Create a discussion:

```
gh api graphql -f query='
  mutation {
    createDiscussion(input: {
      repositoryId: "REPO_ID",
      categoryId: "CATEGORY_ID",
      title: "TITLE",
      body: "BODY"
    }) {
      discussion { number url }
    }
  }'
```

Reply to a discussion:

```
gh api graphql -f query='
  mutation {
    addDiscussionComment(input: {
      discussionId: "DISCUSSION_NODE_ID",
      body: "BODY"
    }) {
      comment { id }
    }
  }'
```

Note: DISCUSSION_NODE_ID is the GraphQL node ID, not the discussion number.
Include `id` in the discussion query's `nodes` to retrieve it.

## Git operations

Branch naming: `issue-{number}-{short-slug}`

Create a worktree branch:

```
git worktree add ../agentlog-issue-NUMBER -b issue-NUMBER-short-slug
```
