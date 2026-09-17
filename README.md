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
compose:                        # optional, for service aliases
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
    service: tools              # compose service...
    path_mapping:
      .: /app                   # host paths are relative to the project root
  node:
    container: my-node          # ...or a plain container (mutually exclusive)
    path_mapping: {assets: /assets/build}
    user: 1001                  # overrides global
    env:
      vars: {NODE_ENV: dev}     # merged over global vars
      deny: [BAZ]               # appended
```

### Path translation

Arguments name host paths, and dockshim makes them usable in the container:
- **Paths under a `path_mapping`** are rewritten: `/home/me/proj/src/a.php` becomes `/app/src/a.php`. Relative paths are kept as typed when the working directory already resolves them.
- **Other existing files** are copied into `/tmp/dockshim-<random>/` in the container for the duration of the command, then removed. Changes made to a copy are not brought back.
- **Directories are not copied**, and neither are `/dev`, `/proc` and `/sys`. Such an argument keeps its value, so it designates the container's own path. Except for the virtual filesystems, dockshim warns when this happens.

```yaml
global:
  path_translation:
    enabled: true               # default
    exclude: [/usr/local/etc]   # paths that should mean the container's own
    max_copy_mb: 100            # default; larger files are left untouched
```

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
| `dockshim config [alias]` | Print the resolved configuration |
| `dockshim validate` | Validate the configuration |
| `dockshim run <alias> [args]` | Run an alias without its shim |

Design decisions are in [docs/adr](docs/adr).
