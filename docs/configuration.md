# Configuration

The config lives in `.dockshim.yaml` or `.dockshim/config.yaml` (`.yml` also works). The directory that holds it is the project root. dockshim looks for it upwards: from the shim's own directory in alias mode, and from the current directory for manager commands. `--config` overrides that search.

Unknown keys are errors, and `dockshim validate` reports every problem at once.

## Reference

```yaml
bin_dir: .dockshim/bin          # where `dockshim install` creates entry points (default)

compose:                        # passed to every docker compose call (optional)
  files: [compose.yaml]         # relative to the project root
  project_name: my-project

global:                         # defaults for every alias
  user: host                    # default: uid:gid of the caller. Or 1000, "1000:1000", www-data
  env:
    deny: [FOO]                 # appended to the built-in denylist
    deny_prefixes: [MISE_]
    allow: [SSH_AUTH_SOCK]      # overrides any deny
    vars: {APP_ENV: dev}        # set in the container
  path_translation:             # see paths.md
    enabled: true
    allow: []
    follow_symlinks: false
    max_copy_mb: 100

aliases:
  php:                          # the alias name is the command run in the container
    service: app                # compose service (required)
    path_mapping:               # optional: inferred from bind mounts when absent, see paths.md
      .: /app
  node:
    service: node
    user: 1001                  # overrides global
    env:
      vars: {NODE_ENV: dev}     # merged over global vars
      deny: [BAZ]               # appended
```

In an alias, scalars override `global`, lists are appended and `vars` are merged.

## Entry points

`dockshim install` creates one `/bin/sh` script per alias in `bin_dir`. It runs `dockshim run --shim`, finding `dockshim` through `PATH`, so upgrading or moving the binary needs no reinstall. `dockshim` must therefore be in the `PATH` of whatever runs the shims, IDEs included.

The scripts are generated: don't edit them, change the config and run `install` again.

## Environment forwarding

Every host variable is forwarded by name (`--env NAME`), so values don't appear in `ps`. These are never forwarded, because their values describe the host:
- names: `PATH HOME PWD OLDPWD TMPDIR TMP TEMP USER LOGNAME HOSTNAME SHELL SHLVL _ LS_COLORS`
- prefixes: `COMPOSE_ DOCKER_ SSH_ XDG_ WSL BASH_FUNC_`

The built-in rules, then `global`, then the alias add to these lists. `allow` wins over any deny. `vars` override host values and bypass the deny rules.

## Interpolation

Values (not keys) are interpolated from the environment, with Compose's syntax:

```yaml
global:
  user: ${APP_UID:-1000}        # default when unset or empty (${VAR-x}: when unset only)
aliases:
  php:
    service: ${PHP_SERVICE:?set PHP_SERVICE}   # custom error (${VAR?msg}: when unset only)
    env:
      vars:
        PS1: $$ ${USER}         # $$ is a literal $
```

A variable that is unset and has no default is an error. Use `${VAR:-}` to allow it to be empty. A variable's value never changes the structure of the YAML.
