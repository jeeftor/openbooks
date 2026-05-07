<h1 align="center">OpenBooks ABS</h1>

<p align="center">
  A focused OpenBooks fork for preparing EPUB libraries for Audiobookshelf.
</p>

OpenBooks ABS is a focused fork of [OpenBooks](https://github.com/evan-buss/openbooks). The original project is a general-purpose IRC ebook search and download tool. This fork keeps that core workflow, but reshapes the server mode around building a clean ebook library that can be mounted directly into [Audiobookshelf](https://www.audiobookshelf.org/).

[Audiobookshelf](https://www.audiobookshelf.org/) works best when books are stored in predictable author/title folders. For EPUB libraries, this fork is designed to turn IRC downloads into layouts like:

```text
Author/Series/Title/Title.epub
Author/Title/Title.epub
```

The goal is simple: search from the browser, download from IRC, clean the EPUB, choose the final name, and leave the finished file in a library folder that Audiobookshelf can scan.

OpenBooks ABS is not affiliated with Audiobookshelf.

## Table of Contents

- [How This Fork Differs](#how-this-fork-differs)
- [Docker](#docker)
  - [Recommended: Calibre Image](#recommended-calibre-image)
  - [Custom Server Command](#custom-server-command)
- [Docker Compose](#docker-compose)
  - [Running Beside Audiobookshelf](#running-beside-audiobookshelf)
- [Important Flags](#important-flags)
- [Post-Processing](#post-processing)
- [Reverse Proxy Base Path](#reverse-proxy-base-path)
- [Local Development](#local-development)
- [Technology](#technology)

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./.github/home_v3_dark.png">
  <img alt="openbooks-abs screenshot" src="./.github/home_v3.png">
</picture>

## How This Fork Differs

OpenBooks ABS is disk-first. Upstream OpenBooks historically focused on delivering downloaded files back to the browser, with persistence as an option. This fork removes that browser-download-centered workflow: downloads are saved to the configured directory, and the browser UI is used to search, monitor, rename, and organize.

Important differences from upstream:

- Better mobile support in the browser UI.
- Multiple browser sessions can use the app at the same time, each with its own IRC connection and download flow.
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

Two image variants are published to GitHub Container Registry. Each variant is available under the new `openbooks-abs` image name and the backwards-compatible `openbooks` image name:

| Tag | Description |
|-----|-------------|
| `ghcr.io/jeeftor/openbooks-abs:latest` | Minimal image. Saves downloads to disk with no default post-processing. |
| `ghcr.io/jeeftor/openbooks:latest` | Backwards-compatible alias for the minimal image. |
| `ghcr.io/jeeftor/openbooks-abs:latest-calibre` | Includes Calibre CLI tools and runs `ebook-polish` on downloaded EPUBs by default. |
| `ghcr.io/jeeftor/openbooks:latest-calibre` | Backwards-compatible alias for the Calibre image. |

Semver releases follow the same pattern for both image names:

```text
ghcr.io/jeeftor/openbooks-abs:v1.2.3
ghcr.io/jeeftor/openbooks:v1.2.3
ghcr.io/jeeftor/openbooks-abs:v1.2.3-calibre
ghcr.io/jeeftor/openbooks:v1.2.3-calibre
```

### Recommended: Calibre Image

The Calibre image is the easiest way to use OpenBooks ABS as an Audiobookshelf intake tool:

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks-abs:latest-calibre
```

By default, the Calibre image starts the server with:

- `--dir /books`
- `--port 80`
- `--post-process-cmd ebook-polish,...`
- `--dev`
- `--name openbooks_abs`

That means the cleaned EPUB is saved normally, and the original pre-polish EPUB is kept beside it as `.orig.epub` for comparison.

### Custom Server Command

Use the minimal image when you want full control over post-processing:

```bash
docker run -p 8080:80 \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks-abs:latest \
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
  ghcr.io/jeeftor/openbooks-abs:latest \
  server \
  --name my_irc_name \
  --dir /books \
  --port 80 \
  --post-process-cmd "my-script,--arg1"
```

## Docker Compose

Minimal openbooks-abs compose:

```yaml
services:
  openbooks-abs:
    image: ghcr.io/jeeftor/openbooks-abs:latest-calibre
    container_name: openbooks-abs
    ports:
      - "8080:80"
    volumes:
      - ./books:/books
    restart: unless-stopped
    environment:
      - BASE_PATH=/
```

Mount the same `./books` directory into Audiobookshelf as an ebook library, then scan it from Audiobookshelf after downloads complete.

### Running Beside Audiobookshelf

In a homelab compose stack, the useful part is the shared volume. OpenBooks ABS writes completed EPUBs into the same host directory that Audiobookshelf sees as an ebook library.

```yaml
services:
  openbooks-abs:
    image: ghcr.io/jeeftor/openbooks-abs:latest-calibre
    container_name: openbooks-abs
    environment:
      - BASE_PATH=/
    volumes:
      - ./books:/books
    ports:
      - "8080:80"
    restart: unless-stopped

  audiobookshelf:
    image: ghcr.io/advplyr/audiobookshelf:latest
    container_name: audiobookshelf
    volumes:
      - ./audiobookshelf/config:/config
      - ./audiobookshelf/metadata:/metadata
      - ./books:/books
    ports:
      - "13378:80"
    restart: unless-stopped
```

Then in Audiobookshelf:

1. Open Audiobookshelf and create an ebook library.
2. Point that library at `/books`.
3. Use OpenBooks ABS to search and download a book.
4. Choose an organized rename option, such as `Author / Series / Title / Title.epub` or `Author / Title / Title.epub`.
5. Scan the Audiobookshelf library.

Because both containers mount `./books`, a book saved by OpenBooks ABS to `./books/Author/Title/Title.epub` is immediately present inside Audiobookshelf at `/books/Author/Title/Title.epub`. Audiobookshelf will pick it up on the next manual or scheduled library scan.

## Important Flags

| Flag | Description |
|------|-------------|
| `--name` | IRC username. Required when you override the default Docker command. |
| `--dir` | Directory where books are saved. Use the directory mounted into Audiobookshelf. |
| `--organize-downloads` | Legacy compatibility flag for organized download workflows. Final placement is chosen in the rename prompt. |
| `--replace-space` | Replace spaces in generated folder names, for example `.` or `_`. |
| `--post-process-cmd` | Command to run after each book download. The downloaded file path is appended as the final argument. |
| `--dev` | Preserve the raw download beside the cleaned file as `name.orig.ext`. Useful when validating `ebook-polish`. |
| `--basepath` | Serve the web UI under a reverse-proxy subpath, such as `/openbooks-abs/`. |
| `--rate-limit` | Seconds between IRC search requests. Minimum is 10. |
| `--searchbot` | IRC search bot name. Defaults to `search`; try `searchook` if needed. |

## Post-Processing

Post-processing is intentionally generic. The server runs the configured command after the book is downloaded and before the final rename/save flow. The downloaded file path is appended automatically.

The Calibre image uses `ebook-polish`, for example:

```bash
--post-process-cmd "ebook-polish,--embed-fonts,--subset-fonts,--smarten-punctuation,--upgrade-book,--remove-unused-css,--compress-images,--add-soft-hyphens"
```

You can replace that with your own script if you want to run validation, conversion, metadata checks, or other cleanup.

Annotated compose example:

```yaml
services:
  openbooks-abs:
    image: ghcr.io/jeeftor/openbooks-abs:latest-calibre
    container_name: openbooks-abs
    environment:
      - BASE_PATH=/
    volumes:
      # Completed books land here. Mount the same path into Audiobookshelf.
      - ./books:/books

      # Optional: mount helper scripts if you want a custom post-processor.
      - ./scripts:/scripts:ro
    ports:
      - "8080:80"
    restart: unless-stopped

    # Override the image default if you want a lighter Calibre cleanup.
    # The downloaded file path is appended automatically as the final argument.
    command: >
      server
      --name openbooks_abs
      --dir /books
      --port 80
      --dev
      --post-process-cmd "ebook-polish,--smarten-punctuation,--upgrade-book,--remove-unused-css"

    # Other useful options:
    #
    # Conservative metadata/container cleanup:
    # --post-process-cmd "ebook-polish,--upgrade-book"
    #
    # More aggressive EPUB cleanup:
    # --post-process-cmd "ebook-polish,--embed-fonts,--subset-fonts,--smarten-punctuation,--upgrade-book,--remove-unused-css,--compress-images,--add-soft-hyphens"
    #
    # Custom script. OpenBooks ABS appends the downloaded file path:
    # --post-process-cmd "/scripts/cleanup-epub.sh,--strict"
```

## Reverse Proxy Base Path

OpenBooks ABS can run behind a reverse proxy at a subpath. The base path must include leading and trailing slashes.

```bash
docker run -p 8080:80 \
  -e BASE_PATH=/openbooks-abs/ \
  -v ./books:/books \
  ghcr.io/jeeftor/openbooks-abs:latest-calibre
```

For binaries or explicit commands:

```bash
./openbooks server --basepath /openbooks-abs/ --name my_irc_name --dir ./books
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
