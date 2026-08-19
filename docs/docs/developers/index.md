# Development

OpenBooks ABS uses a [Makefile](https://github.com/jeeftor/openbooks/blob/master/Makefile) for common development tasks.

## Setup

`make install`

: Installs Go and NPM dependencies.

`make dev-mock`

: Starts a mock IRC / DCC server on `:6667` that mimics basic requests / responses from IRC Highway. Use this for local development to avoid making real requests. See [IRC notes](../irc-notes.md).

## Server Mode Development

Run the following commands in separate terminals.

`make dev1`

: Starts the Vite dev server with hot reload (Vue 3 frontend).

`make dev2`

: Compiles and runs the Go backend in server mode. Connects to the mock IRC server on `localhost:6667`.

## Technology

- **Backend:** Go, Chi router, gorilla/websocket, embedded SPA
- **Frontend:** Vue 3, Pinia, Tailwind CSS, TanStack Virtual, Lucide icons
- **MCP:** Model Context Protocol server for AI agent integration
