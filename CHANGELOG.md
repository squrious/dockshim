# Changelog

## [0.1.0](https://github.com/squrious/dockshim/compare/v0.0.1...v0.1.0) (2026-10-11)

Entry points become relocatable `/bin/sh` scripts, the config moves to the project root, and the CLI keeps only what the shims need.

### ⚠ Upgrading from 0.0.1

1. Move the config to the project root, and drop the old gitignore: `install` writes a new one, but leaves an existing one alone.
   ```bash
   git mv .dockshim/config.yaml .dockshim.yaml
   git rm .dockshim/.gitignore
   ```
2. Update the config:
   - Rename `global:` to `defaults:`.
   - Move `env.vars` to `env`, and `env.deny` and `env.allow` to `env_forwarding.deny` and `env_forwarding.allow`.
   - Write each `env.deny_prefixes` entry as a pattern in `env_forwarding.deny`: `deny_prefixes: [X_]` becomes `deny: [X_*]`.
   - Remove `bin_dir`, `shim_mode`, `path_translation.follow_symlinks` and `path_translation.max_copy_mb`.
3. Run `dockshim install`. It replaces the old entry points.

### ⚠ Breaking changes

* Entry points are always `/bin/sh` scripts in `.dockshim/bin`. Symlink entry points, `shim_mode` and `bin_dir` are removed.
* Aliases only run through their shims: `dockshim run <alias>` is removed.
* A shim only uses the config of the project holding its `.dockshim/bin`. It no longer looks above that project, or falls back to the current directory. A link to a shim works, a copy elsewhere doesn't.
* `--config`/`-c` is removed: manager commands find the config upwards from the current directory.
* `dockshim validate` is removed: every command reports config errors, `dockshim config` included.
* `init --dir`, `--flat` and `--force` are removed. `init` writes `.dockshim.yaml` in the current directory, and refuses when a config exists.
* `dockshim config` takes no arguments, and `--full` is removed: it prints the summary only.
* `install` owns `.dockshim/bin`: it removes anything there that isn't an alias entry point, and refuses to run when `.dockshim` or `.dockshim/bin` is a symlink.
* `env` values are passed by value (`--env NAME=value`), so they show in `ps`: forward secrets from the host by name instead.
* Symlinks count where they point: to copy a link, allow its target's directory. Copied files are no longer size-capped.
* A shim whose alias is gone no longer offers to delete itself: it fails and suggests `dockshim install`.
* Commands no longer warn when the service stops during a command, or when the files copied into the container can't be removed after it.

### Features

* Install script, with checksum verification, `DOCKSHIM_VERSION` and `DOCKSHIM_INSTALL_DIR`. Installation through mise's GitHub backend is documented.
* Entry points find `dockshim` through `PATH`: upgrading or moving the binary, or moving the project, needs no reinstall.
* `.dockshim/` only holds generated files and git-ignores itself: only `.dockshim.yaml` is committed.
* `env_forwarding.deny` and `allow` take `*` patterns: `deny: ['*']` with an `allow` list forwards only that list. Variable names may hold any character but `=`.
* Config errors name the key path and line, and are all reported at once.
* `dockshim config` shows the inferred path mappings and skipped mounts, also while the service is down, and warns about out-of-date shims and aliases whose service isn't in the compose project.
* `install` only checks the config's structure and alias names: an unset variable doesn't stop it.
* Colored output and help on terminals. `NO_COLOR` turns colors off.

### Bug fixes

* `env` values no longer reach docker's own environment: an `env` setting `HOME` or `DOCKER_HOST` used to change the docker CLI's config, contexts and credential helpers.

## 0.0.1 (2026-09-21)

First public release.

### Features

* Run commands in Docker Compose services through per-alias entry points: symlinks, or `/bin/sh` wrappers for platforms without usable symlinks.
* Config in `.dockshim.yaml` or `.dockshim/config.yaml`, discovered from the shim's location or the current directory, with strict validation and Compose-like environment interpolation.
* Manager commands: `init`, `install` (prunes stale entry points), `config`, `validate`, `run`, `version`.
* Auto-start of the compose service, with transparent stdio, TTY, signals and exit codes.
* Environment forwarding by name, with a built-in denylist of host-specific variables, configurable deny/allow rules and injected vars.
* `user: host` default, so files created in the container belong to the caller.
* Path mappings for the working directory and arguments, inferred from the service's bind mounts when not configured.
* Path translation: mapped paths are rewritten, and files in temporary or allowed directories are copied into the container for the duration of the command.
* WSL: Windows drive and `\\wsl.localhost` paths passed by Windows tools are translated.
