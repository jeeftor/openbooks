# Docker

The OpenBooks ABS docker image runs [Server Mode](../modes/server.md). Multi-platform images are published to GitHub Container Registry for each release.

## Docker Compose

For advanced configuration, I recommend using Docker Compose to keep track of container setup.

```yaml title="docker-compose.yml"
version: "3.3"
services:
  openbooks:
    container_name: OpenBooks
    image: ghcr.io/jeeftor/openbooks-abs:latest
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - "~/Downloads/openbooks:/books"
    environment:
      - BASE_PATH=/openbooks/
```

## Configuration

See the [configuration docs](../configuration.md) for a complete list of Server mode configuration options. Pass the configuration flags into the `command` property.

Use the `environment` property to optionally set a custom base path for the server.

## Image Tags

`ghcr.io/jeeftor/openbooks-abs:latest`

: The majority of users will want this image and will always be up to date with the latest release. Note that auto-updating between versions could break configuration.[^1]

`ghcr.io/jeeftor/openbooks-abs:X.X.X`

: Version specific tags. Each time a new release is cut, a new version tagged image is published.

`ghcr.io/jeeftor/openbooks-abs:latest-calibre`

: Includes Calibre tools (ebook-polish) for post-processing.

## Image Platforms

- `linux/amd64`
- `linux/arm64`

[^1]: Tools like [Watchtower](https://containrrr.dev/watchtower/) can check for updates, pull images, and restart containers automatically.
