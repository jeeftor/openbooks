---
name: openbooks-issue-flow
description: Guide Codex through the OpenBooks issue-first GitHub workflow. Use when creating, selecting, updating, or working from issues before code or documentation changes in this repository, including branch/worktree setup, sub-agent parallelization, tests, docs, changelog, PR cleanup, and release preparation.
---

# OpenBooks Issue Flow

## Core Rule

Track every code or documentation change with a GitHub issue before editing files. If the work already has an issue, use it. If not, create one with goal, motivation, and acceptance criteria.

## Start Work

1. Check the worktree with `git status --short --branch`.
2. Confirm the base branch is current `master`; fetch or fast-forward when needed.
3. Create or reuse a GitHub issue before edits.
4. Create a dedicated branch from `master` named for the issue, such as `feature/<short-topic>`, `fix/<short-topic>`, `docs/<short-topic>`, `release/<version>`, or `spike/<short-topic>`.
5. If other work is dirty or unrelated, do not mix it into the issue branch. Park it with a focused commit/PR if it belongs to active work, or ask before taking risky action.

## Parallel Work

Use sub-agents when the user asks for parallelization, multiple issues, research spikes, or independent implementation slices.

- Give each coding worker a separate `git worktree` and a disjoint branch.
- State that workers are not alone in the codebase and must not revert others' changes.
- Assign ownership by issue, files, or responsibility.
- Keep dependency order explicit. If task B depends on task A, make B research/design until A's contract is merged.
- Prefer explorer agents for read-only repo questions and worker agents for bounded code changes.
- When a sub-agent stalls, inspect its worktree read-only, ask for status once, then close it and take over if needed.

## During Implementation

1. Keep edits surgical and tied to the issue acceptance criteria.
2. Match existing project style.
3. Decide whether tests, docs, and `CHANGELOG.md` need updates. Include needed updates in the same branch.
4. During normal feature/fix work, add user-visible changelog entries under the top `## Unreleased` section.
5. Update `README.md` or related docs when usage changes, including Docker images, flags, compose examples, reverse proxy behavior, post-processing, or local development commands.
6. Comment on the issue for meaningful scope decisions, blockers, verification results, and follow-up work.
7. For frontend-visible changes, protect mobile layout: check truncation, wrapping, overlap, and compact controls.

## Verification

Prefer repo-native checks:

- Broad check: `make test` when it covers the touched areas.
- Go changes: `GOCACHE=/private/tmp/openbooks-go-cache rtk go test ./...`
- Frontend changes: `npm --prefix server/app run build`
- Type/lint checks: `make type-check` or `make lint` when relevant.
- Patch hygiene: `git diff --check`
- Shell changes: run `shellcheck` when available.

If a check cannot run, explain why in the issue and PR. Do not claim rendered UI validation if Playwright or a browser runtime is unavailable.

## Pull Request

Open PRs into `master`. Include:

- `Closes #<issue-number>`
- short user-visible summary
- tests run
- docs and changelog note, or why they were not needed
- follow-up issues created, or `none`
- mobile note for UI changes

After a PR is clean and ready, merge it with the repo's existing merge-commit style unless the user asks otherwise. Delete the remote feature branch and remove local branches or worktrees after merge.

## Release Flow

Use this release sequence:

1. Create a release issue.
2. Create `release/<version>` from `master`.
3. Convert the top `Unreleased` changelog section into `## vX.Y.Z - YYYY-MM-DD`.
4. Keep concise user-facing release notes grouped under `Added`, `Improved`, and `Fixed` only where useful.
5. Run relevant checks before opening the release PR.
6. Merge the release PR into `master`.
7. Tag the merged commit with `vX.Y.Z`.
8. Create the GitHub release from that tag using the changelog section as release notes.
9. Remember that `v*.*.*` tags trigger `.github/workflows/ghcr.yml` to publish minimal and Calibre GHCR images with semver tags.
10. Delete the release branch and clean local worktrees.

If the latest changelog version, Git tag, and GitHub release do not agree, state the assumption before publishing.
