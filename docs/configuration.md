# Configuration

The config lives in `.dockshim.yaml` (`.dockshim.yml` also works, but not both). The directory that holds it is the project root. `install` and `config` look for it upwards from the current directory. A shim only uses the config of its own project, the one holding its `.dockshim/bin`, wherever it is called from: a link to a shim works, a copy elsewhere doesn't.

Unknown keys are errors, and commands report all the problems they find at once. `install` only checks the structure and the alias names: values don't change the shims it writes, so they are checked by `config` and by the shims when they run.

## Reference

```yaml
compose:                        # passed to every docker compose call (optional)
  files: [compose.yaml]         # relative to the project root
  project_name: my-project

defaults:                       # inherited by every alias
  user: host                    # default: uid:gid of the caller. Or 1000, "1000:1000", www-data
  env:                          # set in the container, by value
    APP_ENV: dev
  env_forwarding:               # host variables forwarded by name
    deny: [FOO, MISE_*]         # appended to the built-in denylist. * matches anything
    allow: [SSH_AUTH_SOCK]      # overrides any deny
  path_translation:             # see paths.md
    enabled: true
    allow: []

aliases:
  php:                          # the alias name is the command run in the container
    service: app                # compose service (required)
    path_mapping:               # optional: inferred from bind mounts when absent, see paths.md
      .: /app
  node:
    service: node
    user: 1001                  # overrides defaults
    env:
      NODE_ENV: dev             # merged over the defaults' env
    env_forwarding:
      deny: [BAZ]               # appended
```

In an alias, scalars override `defaults`, lists are appended and `env` is merged.

## Entry points

`dockshim install` creates one `/bin/sh` script per alias in `.dockshim/bin`. It runs `dockshim run --shim`, finding `dockshim` through `PATH`, so upgrading or moving the binary needs no reinstall. `dockshim` must therefore be in the `PATH` of whatever runs the shims, IDEs included.

The scripts are generated: don't edit them, change the config and run `install` again. Aliases only run through their shims: `dockshim run`, which they call, is not meant to be run by hand.

`.dockshim/` only holds generated files: `install` writes a `.dockshim/.gitignore` that ignores it all. `.dockshim/bin` belongs to dockshim: `install` makes it contain exactly one script per alias, and removes anything else.

`dockshim config` warns when `.dockshim/bin` differs from the config. A shim whose alias is gone fails and suggests `dockshim install`.

## Environment forwarding

Every host variable is forwarded by name (`--env NAME`), so a secret stays out of `ps`. These are never forwarded, because their values describe the host:
`PATH HOME PWD OLDPWD TMPDIR TMP TEMP USER LOGNAME HOSTNAME SHELL SHLVL _ LS_COLORS COMPOSE_* DOCKER_* SSH_* XDG_* WSL* BASH_FUNC_*`

`deny` and `allow` take names, where `*` matches any run of characters: `X_*` matches `X_` and anything after it, `'*_TOKEN'` any name ending in `_TOKEN`, `*` alone everything. `*` is always a wildcard, so a name containing one can't be matched exactly. `deny: ['*']` with an `allow` list forwards only that list.

A name can hold any character but `=`. Shims are `/bin/sh` scripts, so a shell that drops names that aren't shell identifiers (dash, for example) keeps them from being forwarded; `env` is unaffected.

The built-in rules, then `defaults`, then the alias add to these lists. `allow` wins over any deny.

`env` is set in the container by value (`--env NAME=value`): it overrides host values and bypasses the deny rules, but shows in `ps`. To keep a secret out of it, forward it from the host instead. `env` affects neither docker itself nor Compose interpolation, which only see the caller's environment.

## Interpolation

Values (not keys) are interpolated from the environment, with Compose's syntax:

```yaml
defaults:
  user: ${APP_UID:-1000}        # default when unset or empty (${VAR-x}: when unset only)
aliases:
  php:
    service: ${PHP_SERVICE:?set PHP_SERVICE}   # custom error (${VAR?msg}: when unset only)
    env:
      PS1: $$ ${USER}           # $$ is a literal $
```

A variable that is unset and has no default is an error. Use `${VAR:-}` to allow it to be empty. A variable's value never changes the structure of the YAML.

Every shim reads the whole config when it runs: an unset variable fails every alias, wherever the shims run, IDEs included. Set it there, or give it a default.
