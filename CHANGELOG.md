# Changelog

## Unreleased

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
