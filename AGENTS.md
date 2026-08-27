# Agent Instructions

## CRITICAL: Module Path

**The Go module path is `github.com/jeeftor/openbooks`.**

This is our own fork. The upstream `github.com/evan-buss/openbooks` must NEVER appear in:
- Go import paths
- `go.mod`
- Documentation links, config URLs, or any other config
- New code, refactors, or generated code

The `README.md` is the ONLY file that may mention `evan-buss` (to credit the fork origin). No other file should reference it. If you are creating a new file, adding imports, or writing docs, always use `github.com/jeeftor/openbooks`.

## GitHub Workflow

This is a solo-user app. No gitflow, no feature branches, no PRs for normal work.

- **Work directly on `master`.** Commit and push to `master` — no feature branches, no pull requests, no merge ceremony.
- **Issues are optional.** Use GitHub issues to track bigger features or bugs if helpful, but don't create issues just for the sake of process. Small fixes and tweaks don't need an issue.
- **Always update `CHANGELOG.md`** for user-visible changes. Add a concise entry under `## Unreleased` at the top. When ready to release, move `## Unreleased` content to a new versioned section (e.g. `## v0.4.0 - 2026-08-27`).
- **Run repo-native checks before pushing.** `go test`, `go build`, `npm run build` — whatever's relevant to the change. If a check can't run, note why in the commit message.
- **Keep commits focused.** One logical change per commit. Don't mix unrelated cleanup into a feature commit.
- **The CI auto-tags and builds Docker images on every push to `master`.** Pushing to master triggers `ghcr.io/jeeftor/openbooks:latest` and `:latest-calibre` image builds automatically. No manual release step needed.
- If work is paused or deferred, just leave it — come back later.

## Existing Project Rules

- Default branch: `master`.
- Prefer small, verifiable changes that match the existing project style.
- Use `rg` for content searches and `fd` or `find` for file discovery.
- Run the relevant repo-native checks before committing. If a check cannot be run or has known unrelated failures, document that clearly.
