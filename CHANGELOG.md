# Changelog

## Unreleased

## v3.0.38 - 2026-06-21

### Changed

- **`moveFile` delegates to `copyFile`** for the cross-device fallback — eliminates duplicated copy logic.
- **`filterByFormat` passthrough removed** — callers use `FilterResults` directly.
- **Dead V1 search parser removed** — `ParseSearch` and `parseLine` deleted from `core/search_parser.go` (unused since V2 was introduced).
- **`keepRunes` helper extracted** — `normalizeAuthor` and `normalizeTitle` in `mcp/tools.go` now share a single rune-filter helper.
- **`fileSizeMB` uses `formatBytes`** — consistent human-readable size output across the codebase.
- **`MockSession.isTrustedServer` no longer allocates** — iterates the package-level `mockServers` var directly instead of calling `Servers()`.
- **Stale `trusted[]` removed from MCP tool description** — `search_books` description no longer documents the removed field.
- **Leaked child contexts removed from client hub** — `startClientHub` no longer creates throwaway `context.WithCancel` children; closing the send channel is sufficient.

## v3.0.37 - 2026-06-21

### Changed

- **MCP transport switched from SSE to StreamableHTTP:** The MCP server now uses the StreamableHTTP transport (`mcp-go v0.55.0`) instead of the older SSE transport. This is compatible with Hermes and other modern MCP clients. The endpoint remains `/mcp` on the same port as the web UI — no separate port or container needed.
- **MCP search results deduplicated and token-optimised:** `search_books` now filters to EPUB format and online (trusted) servers only, then groups results by normalised author+title. Each unique title returns one representative entry (largest file). Server names are hoisted to a top-level `servers[]` array; each book carries an integer index `s` instead of repeating the full server name. This reduces response size dramatically for large IRC result sets.
- **MCP `list_library` now accepts a `query` filter:** Pass a `query` string to `list_library` to filter results by filename substring, avoiding the large responses that occur with full library listings.
- **`MCP_BASE_URL` env var removed:** No longer needed — StreamableHTTP does not require a base URL hint. Remove it from any Docker Compose files.
- **README updated with full MCP documentation:** Added MCP server section covering stdio and HTTP transports, all flags, tool descriptions, Claude Desktop config, Docker setup, and reverse proxy URL pattern.

## v3.0.36 - 2026-06-07

### Fixed

- **Book modal no longer dismisses on backdrop click:** Clicking outside the rename/save dialog no longer accidentally saves the book with the original IRC filename. The modal must now be explicitly dismissed via one of the action buttons.

### Changed

- **Notifications moved to bottom-right:** Download notifications now appear in the bottom-right corner instead of top-center, so they no longer obscure the book you are currently editing.
- **Download banner stays visible during file transfer:** The "waiting for bot" banner no longer disappears when the DCC transfer starts. It transitions to a "Transferring…" state with a shimmer progress bar and spinning indicator until the download completes.

## v3.0.32 - 2026-06-05

### Changed

- **Staged Books: delete button always visible:** The trash icon on staged book rows is now always visible instead of appearing only on hover, making it easier to dismiss books you don't want to save.

## v3.0.29 - 2026-05-23

### Fixed

- **Server list now auto-refreshes every 30 seconds:** The online server count wasn't updating regularly because IRC only sends NAMES list when joining the channel or when users join/quit. Added a background goroutine that requests fresh NAMES from IRC every 30 seconds, ensuring the server list stays current.

### Changed

- New `refreshServerList()` goroutine started with each IRC connection.

## v3.0.26 - 2026-05-22

### Fixed

- **Multiple browser windows now share the same IRC session correctly:** Previously, opening a second browser window would "steal" the session from the first, causing notifications to stop appearing in the first window. Now each session tracks all connected clients and broadcasts notifications to every window/tab.

### Changed

- `session.client` (single pointer) changed to `session.clients` (map of all connected WebSocket clients).
- All notifications now broadcast to all connected clients via `broadcastToClients()`.
- Rename prompts still go to only one client (arbitrary) to avoid duplicate dialogs.

## v3.0.25 - 2026-05-22

### Fixed

- **Server online status now session-scoped with real-time updates:** Previously, the global shared server list could get out of sync when multiple users were connected. Now each session tracks its own IRC server list with a timestamp, and updates are pushed via WebSocket in real-time. The UI shows a "(stale)" indicator when server list data is older than 2 minutes.

### Changed

- `/servers` API endpoint now returns `{servers: string[], timestamp: string, fresh: boolean}` instead of `{elevatedUsers: string[]}`.
- Server list polling interval reduced from 30s to 60s (WebSocket provides real-time updates).
- New WebSocket message type: `SERVER_LIST` — pushed whenever IRC sends a names list update.

## v3.0.18 - 2026-05-15

### Added

- **Staged books list view:** "Process Now" now opens a scrollable pick-list of all staged books (cover, title, author, IRC filename, staged time) instead of auto-queuing them one-by-one.
- Each staged book row has a **Delete** button with an inline confirmation, permanently removing the file from disk and the staging registry.
- Clicking **Save →** on a row opens the rename modal for just that one book; after saving, you return to the updated list.
- Closing the list (backdrop click or X) leaves all books staged for later.
- New backend message types: `GET_STAGED_LIST`, `STAGED_BOOKS_LIST`, `PROCESS_ONE_STAGED`.

### Changed

- `StagedRenameModal` backdrop click now triggers **Save Later** instead of silently dismissing.
- Delete option removed from the rename modal footer — it now lives exclusively on the list view where context is clearer.

## v3.0.15 - 2026-05-15

### Changed

- Author and title filter inputs now have autocorrect, autocomplete, autocapitalize, and spellcheck disabled so mobile/macOS does not mangle author names and book titles while filtering.
- Search bar placeholder now reflects the active search query while results are displayed (e.g. `Showing: "Susanna Clarke" — type to search again`), making it clear which search produced the current results.

### Fixed

- Download button state no longer bleeds between books when switching searches or deleting history items. Virtual list rows are now keyed by book identity (`book.full`) rather than row position, so Vue correctly destroys and recreates rows when the underlying book changes. Applied to both desktop table and mobile cards.

## v3.0.11 - 2026-05-15

### Changed

- Filter bar (server, format, author, title toggles) now mounts immediately when a search is issued, before results arrive. A spinner is shown in the table body while waiting so filters are always accessible.
- History entries no longer serialise full book result arrays to localStorage. Only `query`, `timestamp`, and `timedOut` are persisted, eliminating multi-MB localStorage writes on every search result (results remain available in-memory for the session).

### Fixed

- Eliminated a redundant full-list filter pass that was computing `hiddenBooks` separately from `matchedBooks` on every keystroke.

## v3.0.10 - 2026-05-15

### Added

- Generate silly readable IRC guest names for server clients when `--name` is not provided, with active-connection collision handling.
- Search requests are now queued per-session instead of rejected when the rate limit is active. Submitting multiple searches in quick succession queues them and fires them automatically one at a time with the configured cooldown between each (default 30 s). Queue position and countdown are shown in the UI.

### Changed

- Publish GHCR release images from semver tags only, with pushes to `master` automatically creating the next patch tag before publishing.
- Default search rate limit raised from 10 s to 30 s to reduce IRC bot strain.
- The search box is no longer disabled while a search is in-flight. The full-screen 60 s countdown spinner is replaced with a small non-blocking indicator in the stats bar, so you can keep searching without waiting.

### Fixed

- Download button state now persists correctly across filter and group-toggle changes. Previously, changing a filter could reset a "Downloading…" button back to "Download", or incorrectly mark a different book as already clicked due to virtual-list row recycling.
- Clicking a pending (already-queued) history item no longer re-queues the search — it just switches the view to that item.
- Search results now route to the correct history item even if the user navigated to a different item while waiting for results.

## v2.0.2 - 2026-05-08

### Added

- Show release builds as clickable version links to the matching GitHub release notes while keeping dev builds as plain labels.
- Show a compact update indicator next to the UI version when a newer release is available.
- Added a repo-local OpenBooks issue flow skill for issue, branch, PR, changelog, release, and parallel worktree guidance.

### Fixed

- Updated browser favicon assets and metadata to match the bundled Audiobookshelf icon colors.

## v2.0.1 - 2026-05-08

### Fixed

- Improved search result parsing for additional IRC result formats, including Ashurbanipal metadata rows, uppercase extensions, PDB/M4B/OPF formats, filename-only archive rows, and author/title separators without a following space.
