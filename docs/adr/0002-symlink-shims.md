# 2. Alias entry points: symlinks or wrapper scripts

Status: accepted (2026-09-16, amended 2026-09-17 with the wrapper mode)

## Decision
- `dockshim install` creates one entry point per alias in `bin_dir` (default `.dockshim/bin`), in the mode given by `shim_mode`, globally or per alias:
  - **`symlink`** (default): `<bin_dir>/<alias> -> <dockshim executable>`, and `basename(argv[0])` gives the alias name. When that name is `dockshim`, the manager CLI runs instead.
  - **`wrapper`**: a `/bin/sh` script, for filesystems and platforms without usable symlinks (Windows). It runs `exec <dockshim> run --shim "$0" <alias> "$@"`.
- Both modes end in the same code path, with the same arguments, the same discovery anchor and the same stale-shim handling. `--shim` is the hidden equivalent of `argv[0]`, and `$0` is the script's own path. The only difference is that `sh` reorders the environment it passes on, which changes nothing.
- A wrapper is recognized by a marker comment, so `install` can prune it, replace it when the mode changes, and report it when stale.
- `install` removes stale entry points of either kind: symlinks pointing to the current executable or to any file named `dockshim` (moved binaries, dangling links), and scripts carrying the marker. It never touches other files.
- When a shim is invoked but its alias is gone from the config, dockshim exits 127:
  - If stdin and stderr are terminals, it first offers to delete the shim. The prompt goes through `/dev/tty`.
  - Only shims inside the project's `bin_dir` are ever offered for deletion.
- A shim outside `bin_dir` (e.g. after `bin_dir` changed) still works if its alias resolves, with a warning.

## Why
- Real files on disk let IDEs point at them. With symlinks, upgrading dockshim needs no regeneration.
- A single sh script is portable enough for Git Bash and WSL, whereas `exec -a` (which would let a script set `argv[0]`) is not POSIX.

## Consequences
- The integration suite runs entirely in both modes (`eachShimMode`), so they cannot drift apart.
- A wrapper embeds the dockshim path, so moving the binary means running `install` again.
