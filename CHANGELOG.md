# Changelog

## Unreleased

### Improved

- **MCP response token efficiency:** Removed redundant summary text from `search_books`, `list_search_results`, and `download_book` responses — the JSON is self-describing (includes `total`, `truncated`, `has_more`, `staged_id`), so the prepended natural-language summaries were duplicating information and wasting ~150 tokens per call. Responses are now pure JSON.
- **MCP tool descriptions tightened:** The `download_book` CRITICAL block is now ~70 tokens (down from ~150) with the same behavioral instructions.
- **EPUBMetadata JSON fields:** Added `omitempty` tags to `Series` and `SeriesIndex` so empty fields are no longer serialized as `"series":"","series_index":""`. Changed JSON field names from PascalCase (`Author`, `Title`, `Series`, `SeriesIndex`) to lowercase (`author`, `title`, `series`, `series_index`) for consistency with the rest of the MCP API. Web UI TypeScript types and Vue components updated to match.
- **Option `isOrganized` field:** Added `omitempty` so `"isOrganized":false` is no longer serialized on non-organized options.
- **MCP `stagedBookResponse`:** Dropped `replace_space` from the agent-facing response — it's internal config the agent never uses.

### Added

- **MCP `confirm_book` clear metadata fields:** `confirm_book` now accepts `clear_series` and `clear_series_index` boolean params. When true, the field is removed from both the filename path and (with `rewrite_metadata=true`) the EPUB's internal OPF — the `calibre:series` / `calibre:series_index` meta tags are stripped entirely. Use when extracted metadata is wrong (e.g. "The Hobbit" tagged as "The Lord of the Rings" series index 0) and the user wants it removed, not just changed. Previously, omitting a field fell back to the extracted value with no way to clear it.

### Improved

- **MCP `download_book` stronger agent guidance:** The `download_book` tool description now explicitly instructs the agent to ask about each metadata field individually (author, title, series, series_index) and offer to clear wrong fields via `clear_series`/`clear_series_index` rather than just presenting save format options.
- **MCP search shows 20 results inline:** `search_books` now returns the top 20 ranked results (up from 10), so broad queries like "Hobbit" show more variants without needing a second call.
- **MCP `list_search_results` pagination:** `list_search_results` now accepts `offset` (default 0) and `limit` (default 20, max 50) parameters instead of dumping the full result set. The response includes `total`, `offset`, `limit`, and `has_more` so the agent can page through results naturally.
- **MCP search titles cleaned:** IRC file annotations (e.g. `(retail)`, `(epub)`, `(illus)`, `(v5)`, `(kepub)`, `(unabridged)`) and trailing file extensions are now stripped from displayed titles for readability. "The Hobbit (illus) (retail) (epub)" displays as "The Hobbit". Edition years like `(2011)` and series info like `[Series 01]` are preserved. The `dl` download string is never affected.
- **MCP search ranking boosts clean titles:** Titles with no IRC annotations get a +3 relevance bonus, so "The Hobbit" ranks above "The Hobbit (illus) (retail) (epub)" when all else is equal.
- **MCP tool call logging:** Every MCP tool call (`search_books`, `download_book`, `confirm_book`, `discard_staged`, `list_staged`, `list_servers`, `list_library`, `list_search_results`) now logs an entry and exit line to stderr via `slog`, with the tool name, key arguments, duration, and outcome (result count, staged ID, saved path, or error). Tool-layer errors are now logged server-side at `ERROR` level instead of only being returned to the agent.
- **MCP activity log mirrored to stderr:** The `🤖 MCP` activity lines (search results count, download status, post-processing, staged ID, saved path) are now written to stderr in addition to the web UI activity feed, so `docker logs` shows the full MCP narrative.

## v3.0.45 - 2026-06-21

### Added

- **MCP download→confirm→organize metadata flow:** `download_book` now downloads to a staging area, runs the post-processor, reads EPUB metadata (author/title/series/series_index), and builds rename `options[]` without saving to the library. The agent presents the metadata to the user for confirmation, then calls the new `confirm_book` tool to move the file to its final organized path (with optional EPUB internal metadata rewrite). New `list_staged` and `discard_staged` tools manage pending downloads.
- **MCP search results ranking:** `search_books` now ranks results by relevance (query word matches in author/title, source count, file size) and returns only the top 10 with `total` and `truncated` fields. New `list_search_results` tool returns the full cached result set from the last search.
- **MCP download progress notifications:** `download_book` now sends `notifications/message` SSE events to the client during the download — "DCC transfer started" when the IRC file transfer begins, and "Download complete" when post-processing and metadata extraction finish. This gives the agent real-time feedback instead of a silent blocking call.
- **Shared `staging` package:** download→rename→organize logic extracted from `server/` into a shared `staging/` package imported by both the web server and MCP server — no duplication.

### Changed

- **MCP `search_books` response shape:** now includes `total` (full deduplicated count) and `truncated` (true when more results are available via `list_search_results`).

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
