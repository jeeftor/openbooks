# OpenBooks Docker

## Image Variants

| Tag | Description |
|-----|-------------|
| `ghcr.io/jeeftor/openbooks:latest` | Minimal distroless image. No post-processing. |
| `ghcr.io/jeeftor/openbooks:latest-calibre` | Includes Calibre CLI. Runs `ebook-polish` on every downloaded EPUB by default. |

Semver tags follow the same pattern: `v1.2.3` and `v1.2.3-calibre`.

All downloads are always saved to the mounted volume — there is no temporary mode.

## Quick Start

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks:latest \
  server --name my_irc_name --dir /books --port 80
```

## With Calibre Polish (recommended)

The calibre image runs `ebook-polish` automatically after each download:

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks:latest-calibre
```

To customise which polish options are applied, override the command:

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks:latest-calibre \
  server --name my_irc_name --dir /books --port 80 \
  --post-process-cmd "ebook-polish,--embed-fonts,--subset-fonts,--smarten-punctuation,--upgrade-book,--remove-unused-css,--compress-images,--add-soft-hyphens"
```

## Docker Compose

```yaml
services:
  openbooks:
    image: ghcr.io/jeeftor/openbooks:latest-calibre
    container_name: openbooks
    ports:
      - "8080:80"
    volumes:
      - books:/books
    restart: unless-stopped
    environment:
      - BASE_PATH=/openbooks/
    command: >
      server
      --name my_irc_name
      --dir /books
      --port 80
      --organize-downloads
      --replace-space .
      --post-process-cmd "ebook-polish,--subset-fonts,--smarten-punctuation,--upgrade-book,--remove-unused-css,--compress-images,--add-soft-hyphens"

volumes:
  books:
```

## Flags

| Flag | Description |
|------|-------------|
| `--name` | IRC username (required) |
| `--dir` | Directory where books are saved (default: `/books`) |
| `--port` | HTTP port to listen on (default: `80`) |
| `--organize-downloads` | Organize into `Author/Series/Title/` subdirectories using EPUB metadata |
| `--replace-space` | Replace spaces in directory names (e.g. `.` or `-`) |
| `--post-process-cmd` | Command to run after each download, file path appended as last arg. Comma-separated: `cmd,--flag1,--flag2` |
| `--basepath` | Base path for reverse proxy (e.g. `/openbooks/`) |
| `--searchbot` | IRC search bot name (default: `search`, fallback: `searchook`) |
| `--rate-limit` | Seconds between searches (minimum 10) |

## Behind a Reverse Proxy

Set the `BASE_PATH` environment variable to your subpath:

```yaml
environment:
  - BASE_PATH=/openbooks/
```
