# Changelog

## [0.1.0](https://github.com/squrious/dockshim/compare/v0.0.1...v0.1.0) (2026-10-04)


### ⚠ BREAKING CHANGES

* a shim outside any project no longer falls back to the config of the current directory. Use `dockshim run <alias>` to run the current project's alias.
* a failing command no longer warns that the service stopped while it ran, and no longer costs an extra docker compose call.
* a stale shim no longer offers to delete itself. Run `dockshim install` to bring .dockshim/bin in line with the config.
* install now removes anything in .dockshim/bin that isn't an alias entry point, and refuses to run when .dockshim or .dockshim/bin is a symlink. Keep your own scripts elsewhere.
* `dockshim validate` is removed. Any command, such as `dockshim config`, reports config errors.
* the global config section is renamed defaults: rename `global:` to `defaults:`.
* path_translation.follow_symlinks and max_copy_mb are removed. A symlink counts where it points: add its target's directory to path_translation.allow to copy it. Copied files are no longer size-capped.
* dockshim config takes no arguments and only prints the summary; read .dockshim.yaml for var values.
* `init --dir` is removed: run `cd dir && dockshim init`. `init --force` is removed: delete the config file first.
* the config moves from `.dockshim/config.yaml` to `.dockshim.yaml` (or `.yml`) at the project root. `bin_dir` and `init --flat` are removed: shims are always in `.dockshim/bin`, and `install` makes `.dockshim/` git-ignore itself.
* `--config`/`-c` is removed. Run dockshim from the project directory.
* replace symlink shims with PATH-resolving wrappers

### Features

* add install script ([c6cecec](https://github.com/squrious/dockshim/commit/c6cecec811fbca20cdda16ad07d52300c784605a))
* discover a shim's config from its own location only ([8c41c63](https://github.com/squrious/dockshim/commit/8c41c6360b70d095418013b7321b110dce9d5fdd))
* drop config --full and the alias argument, show inferred mappings ([17c40dc](https://github.com/squrious/dockshim/commit/17c40dc482716c148162a5148e93b8564e3557ab))
* drop path_translation.follow_symlinks and max_copy_mb ([e6d2377](https://github.com/squrious/dockshim/commit/e6d23770794270af603d9f446aa6714923d0e3a8))
* drop the warning when the service stops during a command ([5d4a348](https://github.com/squrious/dockshim/commit/5d4a34883081b6a546aea459221d0ddffa5dd570))
* install owns .dockshim/bin ([e71554c](https://github.com/squrious/dockshim/commit/e71554cf559498cf873bbcc39d1ed3a26b28a37d))
* move config to .dockshim.yaml and make .dockshim/ generated-only ([fecf8f1](https://github.com/squrious/dockshim/commit/fecf8f1375ee3fc011ce7f491644f254001d845f))
* remove --config ([3994ab0](https://github.com/squrious/dockshim/commit/3994ab0183cef0f3f641bfe04180c9d5a1e138a1))
* remove init --dir and --force ([652b6ef](https://github.com/squrious/dockshim/commit/652b6effb470cf94cebcec10a4d8aef39901097a))
* remove the validate command ([ecd3798](https://github.com/squrious/dockshim/commit/ecd3798df542e253cc997ee23fd0bb7d2b704678))
* rename global to defaults ([61cbd0a](https://github.com/squrious/dockshim/commit/61cbd0a5dbcf58167022d0c8c24fab36b053b141))
* replace symlink shims with PATH-resolving wrappers ([02748be](https://github.com/squrious/dockshim/commit/02748beb7fb83afbf037dc8eb3e1307e85678c62))
* warn on a terminal when shims are out of date, instead of prompting ([77460be](https://github.com/squrious/dockshim/commit/77460bec6cbfda70d52abc51839b14e8abeedc67))

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
