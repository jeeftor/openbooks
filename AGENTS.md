# Agent Instructions

## GitHub Workflow

- Start new work from a GitHub issue in this repository before making code or documentation changes.
- Use a dedicated feature or fix branch for each issue. Branch names should describe the issue scope.
- Keep the issue updated as the work changes: add comments for scope changes, important decisions, blockers, or follow-up work discovered during implementation.
- Keep commits focused on the issue. Do not mix unrelated cleanup or separate features into the same branch.
- When the work is complete, close the issue from the commit message or pull request with GitHub closing keywords such as `Closes #123`.
- If work is paused or deferred, leave the issue open and comment with the current state and next step.

## Existing Project Rules

- Default branch: `master`.
- Prefer small, verifiable changes that match the existing project style.
- Use `rg` for content searches and `fd` or `find` for file discovery.
- Run the relevant repo-native checks before committing. If a check cannot be run or has known unrelated failures, document that clearly.
