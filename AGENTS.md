# Agent Instructions

## GitHub Workflow

- Track all code and documentation changes with a GitHub issue before editing files.
- If an issue already exists, use it. If not, create one with the goal, motivation, and acceptance criteria.
- Create a dedicated branch from `master` for each issue. Use a descriptive branch name such as `feature/download-search-results` or `fix/series-metadata`.
- Keep the issue updated while working. Add comments for scope changes, important implementation decisions, blockers, test results, and follow-up work discovered during implementation.
- Keep commits focused on the issue. Do not mix unrelated cleanup, refactors, or separate features into the same branch.
- **ALWAYS update `CHANGELOG.md`** for every PR. User-visible features, fixes, behavior changes, Docker/runtime changes, and documentation changes must have a concise changelog entry under `## Unreleased` before the PR is merged. No exceptions — if the PR is user-visible, it gets a changelog entry.
- As part of each feature or fix, decide whether tests and docs need updates. If they do, include them in the same branch. If they do not, note why in the PR.
- Before opening a PR, run the relevant repo-native checks. If a check cannot be run or has known unrelated failures, document that in the PR.
- Open a pull request into `master` when the branch is ready. The PR body must include:
  - the issue it resolves, using `Closes #123`
  - a short summary of user-visible changes
  - tests run
  - docs and changelog updated, or a note that they were not needed
  - any follow-up issues created
- Issues should close through the PR merge, not through direct commits to `master`.
- After a PR is merged, delete the remote feature branch and remove the local branch or worktree.
- Do not push directly to `master` for normal feature or fix work.
- If work is paused or deferred, leave the issue open and comment with the current state and next step.

## Existing Project Rules

- Default branch: `master`.
- Prefer small, verifiable changes that match the existing project style.
- Use `rg` for content searches and `fd` or `find` for file discovery.
- Run the relevant repo-native checks before committing. If a check cannot be run or has known unrelated failures, document that clearly.
