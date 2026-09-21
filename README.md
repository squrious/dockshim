# dockshim

[![Release](https://img.shields.io/github/v/release/squrious/dockshim)](https://github.com/squrious/dockshim/releases/latest)
[![CI](https://github.com/squrious/dockshim/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/squrious/dockshim/actions/workflows/ci.yml)
[![Go version](https://img.shields.io/github/go-mod/go-version/squrious/dockshim)](go.mod)
[![License: MIT](https://img.shields.io/github/license/squrious/dockshim)](LICENSE)

Run commands inside Docker Compose services as if they were installed on the host.

`php`, `composer` or `node` then run in your project's containers, both from your shell and from your IDE. The only thing on the host is one small binary. Each alias is an executable in a project directory, so tools that need a path to an interpreter can point at it.

```console
$ php -v            # runs `php -v` in the `app` compose service
$ phpunit tests/    # workdir and path arguments are mapped into the container
```

## Features

- **Transparent shims**: the arguments, stdin/stdout, TTY, signals and exit code behave like a local command.
- **Auto-start**: the compose service starts on first use.
- **Path handling**: host paths in arguments and the working directory are mapped into the container, using mappings inferred from the service's bind mounts. Temporary files, such as the scripts IDEs generate, are copied in.
- **Environment forwarding**: host variables are forwarded, minus a denylist of host-specific ones (`PATH`, `HOME`, `DOCKER_*`...).
- **WSL aware**: Windows paths passed by Windows IDEs are translated.

## Requirements

- Docker with the Compose v2 plugin (`docker compose`).
- Linux, macOS or WSL. On filesystems without usable symlinks, entry points can be `/bin/sh` wrapper scripts.

## Install

Download a binary from the [releases](https://github.com/squrious/dockshim/releases) and put it in your `PATH`, or:

```bash
go install github.com/squrious/dockshim/cmd/dockshim@latest
```

## Quick start

```bash
cd my-project                            # has a compose.yaml with an `app` service
dockshim init                            # writes .dockshim/config.yaml
```

Declare your aliases:

```yaml
# .dockshim/config.yaml
aliases:
  php:
    service: app
  composer:
    service: app
```

```bash
dockshim install                         # creates .dockshim/bin/php and .dockshim/bin/composer
export PATH="$PWD/.dockshim/bin:$PATH"   # or mise `_.path`, direnv `PATH_add`
php -v
```

Keep `.dockshim/bin/` out of git: the entry points reference a local binary. `dockshim init` writes a `.dockshim/.gitignore` for this.

## Commands

| Command | |
|---|---|
| `dockshim init [-d dir] [--flat] [--force]` | Create a starter config in `.dockshim/config.yaml` (`--flat`: `.dockshim.yaml`) |
| `dockshim install` | Create the alias entry points and remove stale ones |
| `dockshim config [alias] [--full]` | Summarise the resolved configuration (`--full`: everything, as YAML) |
| `dockshim validate` | Validate the configuration |
| `dockshim run <alias> [args]` | Run an alias without its shim |
| `dockshim version` | Print the version |

## Documentation

- [Configuration](docs/configuration.md): every option, env forwarding and interpolation.
- [Paths](docs/paths.md): path mapping, path translation and Windows paths under WSL.
- [Development](docs/development.md): building, testing and releasing.
- [Design decisions](docs/adr).

## License

[MIT](LICENSE)
