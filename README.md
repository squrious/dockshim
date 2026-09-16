# dockshim

Run commands inside Docker containers as if they were installed on the host.

```bash
mise run install              # builds ~/.local/bin/dockshim
cd my-project
dockshim install              # creates .dockshim/bin/<alias> symlinks
export PATH="$PWD/.dockshim/bin:$PATH"   # or mise `_.path`, direnv `PATH_add`
php -v                        # runs `php -v` in the `tools` compose service
```

Add `.dockshim/bin/` to the project's `.gitignore`: the symlinks point to a local binary.

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

## Commands

| Command | |
|---|---|
| `dockshim install` | Create shims for all aliases and remove stale ones |
| `dockshim config [alias]` | Print the resolved configuration |
| `dockshim validate` | Validate the configuration |
| `dockshim run <alias> [args]` | Run an alias without its shim |

Design decisions are in [docs/adr](docs/adr).
