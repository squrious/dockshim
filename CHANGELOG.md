# Changelog

## [0.1.0](https://github.com/squrious/dockshim/compare/v0.0.1...v0.1.0) (2026-09-21)


### Features

* allow-list copied paths and translate WSL Windows paths ([8637666](https://github.com/squrious/dockshim/commit/863766626665bc09ea2fca7a81166ce59cacb61a))
* **cli:** add init command ([2e94c8a](https://github.com/squrious/dockshim/commit/2e94c8aa08196de7f20ebf0b1f86c6e6799a361e))
* **config:** interpolate environment variables in values ([371dfd6](https://github.com/squrious/dockshim/commit/371dfd629d37cf0b653b87591f8f79369293b28c))
* infer path mappings from bind mounts, drop retry and container target ([bc8466f](https://github.com/squrious/dockshim/commit/bc8466f057d92eb620a32ad0f34bd1d3e115956e))
* run commands in containers through shims ([3b018cf](https://github.com/squrious/dockshim/commit/3b018cff72c8809168229ae0699e45d8e1f1bb86))
* **shim:** add wrapper shim mode ([2b2ff47](https://github.com/squrious/dockshim/commit/2b2ff471eefd5d65ba5117a04f81bba6f50c23a2))
* translate host paths given as arguments ([bf2b05f](https://github.com/squrious/dockshim/commit/bf2b05f08a01c46786c076e012e636c84649327f))

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
