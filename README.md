# OpenBooks for Audiobookshelf

This repository is a focused fork of [OpenBooks](https://github.com/evan-buss/openbooks). The original project is a general-purpose IRC ebook search and download tool. This fork keeps that core workflow, but reshapes the server mode around building a clean ebook library that can be mounted directly into [Audiobookshelf](https://www.audiobookshelf.org/).

[Audiobookshelf](https://www.audiobookshelf.org/) works best when books are stored in predictable author/title folders. For EPUB libraries, this fork is designed to turn IRC downloads into layouts like:

```text
Author/Series/Title/Title.epub
Author/Title/Title.epub
```

The goal is simple: search from the browser, download from IRC, clean the EPUB, choose the final name, and leave the finished file in a library folder that Audiobookshelf can scan.

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./.github/home_v3_dark.png">
  <img alt="openbooks screenshot" src="./.github/home_v3.png">
</picture>

## How This Fork Differs

This fork is disk-first. Upstream OpenBooks historically focused on delivering downloaded files back to the browser, with persistence as an option. This fork removes that browser-download-centered workflow: downloads are saved to the configured directory, and the browser UI is used to search, monitor, rename, and organize.

Important differences from upstream:

- Better mobile support in the browser UI.
- A rename workflow after download, including metadata-based naming suggestions.
- EPUB metadata and cover extraction before saving.
- Optional EPUB internal metadata rewrite when you confirm a renamed book.
- Rename choices that can save into Audiobookshelf-style author/title paths.
- Configurable space replacement for folder names, such as `Author.Name/Book.Title/`.
- Post-process hooks that run after each download, with the file path appended automatically.
- A prebuilt Calibre Docker image that runs `ebook-polish` on downloaded EPUBs.
- Activity logs for download, cleaning, rename, and save steps.
- A local testing `--dev` mode that preserves the raw download beside the cleaned file as `Title.orig.epub`.

## Docker

Two image variants are published to GitHub Container Registry:

| Tag | Description |
|-----|-------------|
| `ghcr.io/jeeftor/openbooks:latest` | Minimal image. Saves downloads to disk with no default post-processing. |
| `ghcr.io/jeeftor/openbooks:latest-calibre` | Includes Calibre CLI tools and runs `ebook-polish` on downloaded EPUBs by default. |

Semver releases follow the same pattern: `v1.2.3` and `v1.2.3-calibre`.

### Recommended: Calibre Image

The Calibre image is the easiest way to use this fork as an Audiobookshelf intake tool:

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks:latest-calibre
```

By default, the Calibre image starts the server with:

- `--dir /books`
- `--port 80`
- `--post-process-cmd ebook-polish,...`
- `--dev`
- `--name openbooks`

That means the cleaned EPUB is saved normally, and the original pre-polish EPUB is kept beside it as `.orig.epub` for comparison.

### Custom Server Command

Use the minimal image when you want full control over post-processing:

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks:latest \
  server \
  --name my_irc_name \
  --dir /books \
  --port 80 \
  --replace-space .
```

To add your own cleanup command:

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks:latest \
  server \
  --name my_irc_name \
  --dir /books \
  --port 80 \
  --post-process-cmd "my-script,--arg1"
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
      - ./books:/books
    restart: unless-stopped
    environment:
      - BASE_PATH=/
```

Mount the same `./books` directory into Audiobookshelf as an ebook library, then scan it from Audiobookshelf after downloads complete.

## Important Flags

| Flag | Description |
|------|-------------|
| `--name` | IRC username. Required when you override the default Docker command. |
| `--dir` | Directory where books are saved. Use the directory mounted into Audiobookshelf. |
| `--organize-downloads` | Legacy compatibility flag for organized download workflows. Final placement is chosen in the rename prompt. |
| `--replace-space` | Replace spaces in generated folder names, for example `.` or `_`. |
| `--post-process-cmd` | Command to run after each book download. The downloaded file path is appended as the final argument. |
| `--dev` | Preserve the raw download beside the cleaned file as `name.orig.ext`. Useful when validating `ebook-polish`. |
| `--basepath` | Serve the web UI under a reverse-proxy subpath, such as `/openbooks/`. |
| `--rate-limit` | Seconds between IRC search requests. Minimum is 10. |
| `--searchbot` | IRC search bot name. Defaults to `search`; try `searchook` if needed. |

## Post-Processing

Post-processing is intentionally generic. The server runs the configured command after the book is downloaded and before the final rename/save flow. The downloaded file path is appended automatically.

The Calibre image uses `ebook-polish`, for example:

```bash
--post-process-cmd "ebook-polish,--embed-fonts,--subset-fonts,--smarten-punctuation,--upgrade-book,--remove-unused-css,--compress-images,--add-soft-hyphens"
```

You can replace that with your own script if you want to run validation, conversion, metadata checks, or other cleanup.

## Reverse Proxy Base Path

OpenBooks can run behind a reverse proxy at a subpath. The base path must include leading and trailing slashes.

```bash
docker run -p 8080:80 \
  -e BASE_PATH=/openbooks/ \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks:latest-calibre
```

For binaries or explicit commands:

```bash
./openbooks server --basepath /openbooks/ --name my_irc_name --dir ./books
```

## Local Development

```bash
make dev
```

Useful targets:

| Target | Description |
|--------|-------------|
| `make dev` | Backend and frontend together. |
| `make dev1` | Backend only. |
| `make dev2` | Frontend only. |
| `make dev-mock` | Mock IRC/DCC server plus app for local testing. |
| `make docker-calibre` | Build the Calibre image. |
| `make docker-dev-calibre` | Build and run the Calibre image locally. |

## Technology

- Go server
- Chi HTTP router
- gorilla/websocket
- Vue 3
- TypeScript
- Vite
- Tailwind CSS
- Calibre `ebook-polish` in the `latest-calibre` image
