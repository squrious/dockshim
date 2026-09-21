# Changelog

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
