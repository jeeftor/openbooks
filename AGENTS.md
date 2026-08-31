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

## CRITICAL: IRC #ebooks Channel Rules

The app connects to `irc.irchighway.net` and joins `#ebooks` to search for books. This is a real IRC channel run by human operators who **will ban you** if you abuse it. The channel topic literally says "Don't test scripts here!"

### What got us banned (August 2026)

During development, we connected and disconnected 15+ times in rapid succession from the same IP, ran automated search tests, sent test messages to the channel, and used predictable nick patterns (`openbooks_*`, `ob_test_*`).

The channel operator `fruitloops` set these bans:
- `openbooks*!*@*` — bans ALL nicks starting with "openbooks"
- `ob_test*!*@*` — bans our test nicks
- `*!*@ihw-<our-IP>` — IP-based ban (blocks everything from that network)

### Rules to prevent future bans

1. **NEVER use predictable nick prefixes.** The app's guest name generator (`server/guest_names.go`) produces names like `eager_lion` — these are safe. The Makefile no longer sets `--name` by default. Do not change this.
2. **NEVER send test/noise messages to #ebooks.** Don't write tests or scripts that `PRIVMSG #ebooks` with "hello" or "test connection". Only real user-initiated searches should go to the channel.
3. **NEVER write live IRC integration tests.** We removed `core/live_irc_test.go` and the `liveirc` build tag because automated connections to #ebooks risk getting us banned again. Do NOT re-add them. Use the mock IRC server (`cmd/mock_server`) for testing instead.
4. **Minimize connections.** The IRC server has a session limit (~2-3 per IP). Don't start multiple OpenBooks instances from the same IP simultaneously.
5. **The MCP server no longer requires `--name`.** If not set, it generates a random nick like `reader1a2b3c4d`. Do not re-add the `log.Fatal("--name is required")` check.
6. **E2e Playwright tests must not match `openbooks_` text.** The username is a random guest name. Tests should wait for the header connection indicator (non-empty `span.truncate`) instead.
