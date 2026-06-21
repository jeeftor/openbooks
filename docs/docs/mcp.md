# MCP Server

OpenBooks can expose its search and download functionality as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server, allowing AI agents (Claude, etc.) to search for and download ebooks on your behalf.

## Tools

| Tool | Description |
|------|-------------|
| `search_books` | Search IRC for ebooks. Returns structured results filtered to epub (zero-size entries excluded) with server trust, file size, and a download string so the agent can rank and choose the best result. Searches are synchronous and may take up to 60 seconds. |
| `download_book` | Download a specific book using the `download_string` from a search result. |
| `list_servers` | List currently available IRC download servers. |
| `list_library` | List ebooks already downloaded to the local library. |

The agent receives raw structured results and ranks them itself — signals include `trusted_server`, `size`, and title match quality.

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
- Results are filtered to `epub` by default and zero-size entries are always excluded.
- When embedded in the web server, MCP search and download activity appears in the web UI log panel prefixed with `🤖 MCP`.
- The MCP agent uses a separate IRC connection from browser users (username `<name>_mcp`).
