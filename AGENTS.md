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

## IRC #ebooks Channel Rules

The app connects to `irc.irchighway.net` and joins `#ebooks` to search for books. This is a real IRC channel run by human operators. The channel topic says "Don't test scripts here!" — be respectful.

### Incident history (August 2026)

During development, we connected/disconnected 15+ times rapidly from the same IP, ran automated search tests, sent test messages to the channel, and used predictable nick patterns (`openbooks_*`, `ob_test_*`). The channel operator `fruitloops` set nick-pattern and IP bans. The IP ban became moot after switching ISPs. These bans may or may not still be active — check with `MODE #ebooks +b` from a fresh connection if you need to know.

### Permanent best practices

These rules apply regardless of whether any bans are currently active:

1. **Prefer random guest names over fixed `--name` prefixes.** The app's guest name generator (`server/guest_names.go`) produces names like `eager_lion`. The Makefile defaults to no `--name` (random guest name). You can override with `make dev NAME=myuser` but avoid `openbooks_*` patterns.
2. **Don't send test/noise messages to #ebooks.** Only real user-initiated searches should go to the channel. No "hello", "test", or probe messages.
3. **Don't write live IRC integration tests.** Automated connections to #ebooks risk bans. Use the mock IRC server (`cmd/mock_server`) for testing instead. If live tests are ever truly needed, use random non-patterned nicks and minimize connection count — but they were removed deliberately in Aug 2026 and should not be re-added lightly.
4. **Minimize simultaneous connections.** The IRC server has a session limit (~2-3 per IP). Don't start multiple OpenBooks instances from the same IP at the same time.
5. **The MCP server doesn't require `--name`.** If not set, it generates a random nick like `reader1a2b3c4d`. Do not re-add a hard `log.Fatal` requirement.
6. **E2e Playwright tests must not match `openbooks_` text.** The username is a random guest name. Tests should wait for the header connection indicator (non-empty `span.truncate`) instead.
