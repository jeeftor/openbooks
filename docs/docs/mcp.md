# MCP Server

OpenBooks can expose its search and download functionality as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server, allowing AI agents (Claude, etc.) to search for and download ebooks on your behalf.

## Tools

| Tool | Description |
|------|-------------|
| `search_books` | Search IRC for ebooks. Returns the top 10 results ranked by relevance (query word matches in author/title, source count, file size), filtered to epub from trusted servers, deduplicated by title. Response includes `total` and `truncated` so the agent knows if more are available. Synchronous — may take up to 60 seconds. |
| `list_search_results` | Return the full result set from the most recent `search_books` call. Use when `search_books` returned a truncated list and the user wants to see all matches. |
| `download_book` | Download a book using the `dl` string from `search_books`. Downloads to a staging area, runs the post-processor, reads EPUB metadata (author/title/series/series_index), and builds rename `options[]`. Returns `staged_id`, `irc_filename`, `metadata`, and `options` — the file is **not** saved to the library yet. The agent must present the metadata to the user for confirmation. Sends progress notifications (`notifications/message`) during the download: "DCC transfer started" when the file transfer begins, and "Download complete" when post-processing finishes. |
| `confirm_book` | Save a staged book to the library. Pass the `staged_id` from `download_book`, the chosen `option_id` from `options[]`, and the confirmed/edited `author`/`title`/`series`/`series_index`. Set `rewrite_metadata=true` to patch the EPUB's internal OPF metadata to match. Returns the final relative path. |
| `list_staged` | List books downloaded via `download_book` that are still awaiting confirmation. |
| `discard_staged` | Permanently delete a staged book without saving it to the library. |
| `list_servers` | List currently available IRC download servers. |
| `list_library` | List ebooks already downloaded to the local library. Accepts an optional `query` to filter by filename substring. |

### Search → download → confirm flow

```
search_books "dune frank herbert"
  → top 10 ranked results (total=25, truncated=true)
  → agent presents to user; user picks one

download_book dl="!ThrawnBot Frank Herbert - Dune.epub"
  → downloads, cleans, extracts metadata
  → returns staged_id, metadata {author, title, series, series_index}, options[]
  → agent asks user: "Is the author correct? Series?"

confirm_book staged_id=... option_id="organized" author="Frank Herbert" title="Dune"
  → moves file to final organized path
  → optionally rewrites EPUB internal metadata
  → returns "Frank Herbert/Dune.epub"
```

If the user doesn't want the book, call `discard_staged` with the `staged_id`.

## Modes

### Built into the web server (recommended)

Pass `--mcp` when starting the server. OpenBooks connects to IRC with a dedicated MCP session (username `<name>_mcp`) and mounts the MCP endpoint at `/mcp` on the same port as the web UI. No extra port needed.

```shell
openbooks server --name myuser --dir ~/Books --mcp
```

The MCP SSE endpoint will be at `http://127.0.0.1:5228/mcp/sse`.

If the server is reachable from other machines, set `--mcp-base-url` so MCP clients get the correct callback address:

```shell
openbooks server --name myuser --dir ~/Books --mcp --mcp-base-url http://myserver:5228
```

To restrict which file formats are returned to the agent (default is `epub` only):

```shell
openbooks server --name myuser --dir ~/Books --mcp --mcp-formats epub,mobi
```

**Docker** — enable via environment variables, no CMD override needed:

```shell
docker run -p 8080:80 \
  -v ~/Books:/books \
  -e ENABLE_MCP=true \
  -e MCP_BASE_URL=http://myserver:8080 \
  ghcr.io/jeeftor/openbooks:latest-calibre
```

### Standalone MCP server

Run the `mcp` subcommand to start a dedicated MCP process, independent of the web server.

**stdio** (default) — for Claude Desktop / Claude Code as a local subprocess:

```shell
openbooks mcp --name myuser --dir ~/Books
```

**HTTP/SSE** — listen on a port:

```shell
openbooks mcp --name myuser --dir ~/Books --port 8765
```

**Mock mode** — no IRC connection, returns fake data for testing:

```shell
openbooks mcp --mock --port 8765
```

## Environment variables (Docker)

| Variable | Description |
|----------|-------------|
| `ENABLE_MCP` | Set to any non-empty value to enable the MCP endpoint. |
| `MCP_BASE_URL` | External base URL MCP clients use for callbacks, e.g. `http://myserver:8080`. Defaults to `http://127.0.0.1:<port>` which only works for local clients. |

## Claude Desktop configuration

**Standalone stdio** (local binary):

```json
{
  "mcpServers": {
    "openbooks": {
      "command": "openbooks",
      "args": ["mcp", "--name", "myuser", "--dir", "/path/to/books"]
    }
  }
}
```

**SSE** (connecting to a running web server with `--mcp`):

```json
{
  "mcpServers": {
    "openbooks": {
      "type": "sse",
      "url": "http://myserver:5228/mcp/sse"
    }
  }
}
```

## Claude Code configuration

**Standalone stdio:**

```shell
claude mcp add openbooks -- openbooks mcp --name myuser --dir ~/Books
```

**SSE** (connecting to a running server):

```shell
claude mcp add --transport sse openbooks http://myserver:5228/mcp/sse
```

## Notes

- Only one search can be in-flight at a time — concurrent agent calls are serialised automatically.
- `search_books` has a 90-second timeout; `download_book` has a 3-minute timeout.
- `search_books` returns the top 20 results by relevance; call `list_search_results` with an `offset` to page through the rest. The full set is cached server-side from the last search.
- `list_search_results` accepts `offset` (default 0) and `limit` (default 20, max 50) for pagination. The response includes `total`, `offset`, `limit`, and `has_more`.
- IRC file annotations (e.g. `(retail)`, `(epub)`, `(illus)`, `(v5)`) are stripped from displayed titles for readability. Edition years and series info are preserved. The `dl` string is never affected.
- Results are filtered to `epub` by default and zero-size entries are always excluded.
- `download_book` does **not** save to the library — it stages the file and returns metadata for the agent to confirm with the user. Use `confirm_book` to save, or `discard_staged` to cancel.
- When embedded in the web server, MCP search and download activity appears in the web UI log panel prefixed with `🤖 MCP`.
- Every MCP tool call is logged to stderr (`slog`) with the tool name, key arguments, duration, and outcome — visible in `docker logs` as `mcp tool call` / `mcp tool ok` / `mcp tool error` lines. The `🤖 MCP` activity lines are also mirrored to stderr.
- The MCP agent uses a separate IRC connection from browser users (username `<name>_mcp`).
