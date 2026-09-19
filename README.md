# dockshim

Run commands inside Docker containers as if they were installed on the host.

```bash
mise run install              # builds ~/.local/bin/dockshim
cd my-project
dockshim init                 # writes .dockshim/config.yaml, then declare aliases in it
dockshim install              # creates .dockshim/bin/<alias> symlinks
export PATH="$PWD/.dockshim/bin:$PATH"   # or mise `_.path`, direnv `PATH_add`
php -v                        # runs `php -v` in the `tools` compose service
```

Keep `.dockshim/bin/` out of git, since the entry points reference a local binary. `dockshim init` writes a `.dockshim/.gitignore` for this.

### Shim modes

`dockshim install` creates one entry point per alias: a symlink to the dockshim binary (default), or a `/bin/sh` wrapper script for platforms without usable symlinks, such as Windows. Both behave identically.

```yaml
global:
  shim_mode: wrapper        # symlink (default) or wrapper
aliases:
  php:
    shim_mode: symlink      # overrides global
```

## Configuration

Put the config in `.dockshim.yaml` or `.dockshim/config.yaml` at the project root:

```yaml
bin_dir: .dockshim/bin          # default
compose:                        # optional
  files: [compose.yaml]
  project_name: my-project
global:
  user: host                    # default: uid:gid of the caller. Or 1000, "1000:1000", www-data
  env:
    deny: [FOO]                 # appended to the built-in denylist
    deny_prefixes: [MISE_]
    allow: [SSH_AUTH_SOCK]      # overrides any deny
    vars: {APP_ENV: dev}
aliases:
  php:
    service: tools              # compose service
    path_mapping:               # optional, see below
      .: /app                   # host paths are relative to the project root
  node:
    service: node
    user: 1001                  # overrides global
    env:
      vars: {NODE_ENV: dev}     # merged over global vars
      deny: [BAZ]               # appended
```

### Path mapping

`path_mapping` tells which host directories are visible where in the container. It gives the working directory (`--workdir`) and drives path translation.

Without it, dockshim reads the bind mounts of the running container (`docker inspect`) and maps those inside the project directory. Mounts from elsewhere, such as `/var/run/docker.sock`, are skipped silently; `dockshim config` lists them while the service runs. Set `path_mapping`, even to `{}`, to choose the mappings yourself: inference is then off.

> **Limited support:** inference compares the mount sources docker reports with the project path. With a remote daemon, or Docker Desktop when it reports bind sources under its own paths (as it may for WSL distributions), nothing matches and nothing is mapped. Set `path_mapping` explicitly there.

### Path translation

Arguments name host paths, and dockshim makes them usable in the container:
- **Paths under a `path_mapping`** are rewritten: `/home/me/proj/src/a.php` becomes `/app/src/a.php`. Relative paths are kept as typed when the working directory already resolves them.
- **Files in an allowed directory** are copied into `/tmp/dockshim-<random>/` in the container for the duration of the command, then removed. Changes made to a copy are not brought back. Allowed are the system temporary directories and whatever `allow` adds: copying exists for the throwaway files a tool hands a command, such as an IDE test runner script.
- **Anything else** keeps its value, so it designates the container's own path. That is the ordinary case, and it is silent. Directories are never copied, and dockshim warns when a path in an allowed directory cannot be copied: a directory, an unreadable file, or one over the cap.

```yaml
global:
  path_translation:
    enabled: true               # default
    allow: [/srv/fixtures]      # extra directories whose files may be copied
    follow_symlinks: false      # default; true lets a link in an allowed directory point out of it
    max_copy_mb: 100            # default; larger files are left untouched
```

Set `allow: ["/"]` to copy from anywhere.

#### Windows paths (WSL)

A Windows tool driving a shim inside WSL passes Windows paths — PhpStorm running a test gives `C:/Users/me/AppData/Local/Temp/ide-phpunit.php` and `\\wsl.localhost\Ubuntu\home\me\proj\tests`, sometimes spelled `//wsl.localhost/Ubuntu/home/me/proj/tests` in the same command line. dockshim reads both: a mounted drive becomes its mount point (`/mnt/c/...`), and a path in this distribution becomes its plain Linux path, which then translates like any other. Windows temporary directories are allowed like the Linux ones, so the runner script above is copied and the test directory is rewritten through `path_mapping`.

A Windows path this distribution cannot reach — an unmounted drive, another distribution, a network share — is left as typed, with a warning.

### Environment variables

Values (not keys) are interpolated from the environment, with Compose-like syntax:

```yaml
global:
  user: ${APP_UID:-1000}        # default when unset or empty
aliases:
  php:
    service: ${PHP_SERVICE:?set PHP_SERVICE}   # custom error
    env:
      vars:
        PS1: $$ ${USER}         # $$ is a literal $
```

A variable that is unset and has no default is an error. Use `${VAR:-}` to allow it to be empty.

## Commands

| Command | |
|---|---|
| `dockshim init [-d dir] [--flat] [--force]` | Create a starter config in `.dockshim/config.yaml` (`--flat`: `.dockshim.yaml`) |
| `dockshim install` | Create the alias entry points and remove stale ones |
| `dockshim config [alias] [--full]` | Summarise the resolved configuration (`--full`: everything, as YAML) |
| `dockshim validate` | Validate the configuration |
| `dockshim run <alias> [args]` | Run an alias without its shim |

Design decisions are in [docs/adr](docs/adr).
