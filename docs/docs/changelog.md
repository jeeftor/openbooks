## Unreleased

### Added
- MCP (Model Context Protocol) server for AI agent access. Tools: `search_books`, `download_book`, `list_servers`, `list_library`.
- `openbooks mcp` subcommand — standalone MCP server over stdio or HTTP/SSE (`--port`).
- `openbooks server --mcp` — mounts MCP endpoint at `/mcp` on the existing web server port (single port, both UI and MCP).
- `ENABLE_MCP=true` environment variable for enabling MCP in Docker without overriding `CMD`.
- `--mcp-formats` / `--formats` flag to control which file formats the agent sees (default: `epub`).
- `--mock` flag on `openbooks mcp` for testing without a live IRC connection.
- Docs page: MCP Server.

---

# [v4.5.0] - 2023-01-08

## Added
-  Use `--useragent/-u` flag to optionally specify the [UserAgent](https://en.wikipedia.org/wiki/Client-to-client_protocol#VERSION) reported to the IRC server. Default remains `OpenBooks v4.5.0`.

## Breaking 
- `--name/-n` flag **must** be specified when starting the application. OpenBooks will no longer generate a random `noun_adjective` username.
- Only a single connection to the IRC server will be made. Opening a second browser tab will show an error message.


