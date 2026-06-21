# MCP Server

OpenBooks can expose its search and download functionality as an [MCP (Model Context Protocol)](https://modelcontextprotocol.io) server, allowing AI agents (Claude, etc.) to search for and download ebooks on your behalf.

## Tools

| Tool | Description |
|------|-------------|
| `search_books` | Search IRC for ebooks matching a query. Returns structured results with server trust, file size, and a download string. |
| `download_book` | Download a specific book using the `download_string` from a search result. |
| `list_servers` | List currently available IRC download servers. |
| `list_library` | List ebooks already downloaded to the local library. |

The agent receives raw structured results and decides which book to download — it can rank by trusted server status, file size, and title match quality.

## Modes

### Built into the web server (recommended)

Pass `--mcp` when starting the server. OpenBooks connects to IRC with a dedicated MCP session and mounts the MCP endpoint at `/mcp` on the same port as the web UI.

```shell
openbooks server --name myuser --dir ~/Books --mcp
```

The MCP SSE endpoint will be available at `http://127.0.0.1:5228/mcp`.

To restrict which file formats are returned to the agent:

```shell
openbooks server --name myuser --dir ~/Books --mcp --mcp-formats epub,mobi
```

**Docker** — enable via environment variable (no need to override the CMD):

```shell
docker run -p 8080:80 -v ~/Books:/books -e ENABLE_MCP=true evanbuss/openbooks
```

The MCP endpoint will then be at `http://localhost:8080/mcp`.

### Standalone MCP server

Run the `mcp` subcommand to start a dedicated MCP process, independent of the web server.

**stdio** (default) — for Claude Desktop / Claude Code:

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

## Claude Desktop configuration

Add to your `claude_desktop_config.json`:

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

Or, if you are already running the web server with `--mcp`:

```json
{
  "mcpServers": {
    "openbooks": {
      "type": "sse",
      "url": "http://127.0.0.1:5228/mcp/sse"
    }
  }
}
```

## Claude Code configuration

```shell
claude mcp add openbooks -- openbooks mcp --name myuser --dir ~/Books
```

Or for the SSE endpoint (when the server is running with `--mcp`):

```shell
claude mcp add --transport sse openbooks http://127.0.0.1:5228/mcp/sse
```

## Notes

- Only one search can be in-flight at a time. Concurrent agent calls are serialised automatically.
- There is a 90-second timeout per search and 3-minute timeout per download.
- When embedded in the web server (`--mcp`), the MCP agent uses a separate IRC connection from browser users (username suffix `_mcp`).
- The `--mcp-formats` / `--formats` filter defaults to `epub` only.
