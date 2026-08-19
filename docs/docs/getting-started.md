OpenBooks ABS provides a convenient web user interface over [IRC Highway's](https://irchighway.net/) `#ebook` channel.
It streamlines the process of connecting, searching for, and downloading books.

!!! tip "OpenBooks does not host any content, think of it as a single-purpose IRC client."

The primary mode of operation is Server Mode where you search and download via a web interface in your browser.
This allows you to self-host OpenBooks ABS without having to install it on every device.

### Docker

`docker run -p 8080:80 ghcr.io/jeeftor/openbooks-abs:latest`

: Basic configuration that exposes the web interface on [http://localhost:8080](http://localhost:8080) and saves all files to an anonymous volume.

`docker run -p 8080:80 -v ~/Downloads/openbooks:/books ghcr.io/jeeftor/openbooks-abs:latest`

: More advanced configuration that exposes the web interface on [http://localhost:8080](http://localhost:8080) and saves all eBook files to the mounted volume at `~/Downloads/openbooks`.

> For more information see the [docker guide](./setup/docker.md).

### Executable

1. Download the latest release for your platform from the [releases page](https://github.com/jeeftor/openbooks/releases).
2. Execute it from your terminal in Server mode (`./openbooks server`).

   - Linux users may have to run `chmod +x [binary name]` to make it executable

> For more information see the [executable guide](./setup/binary.md).
