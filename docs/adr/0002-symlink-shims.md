# 2. Aliases are symlinks, dispatched on argv[0]

Status: accepted (2026-09-16)

## Decision
- `dockshim install` creates `<bin_dir>/<alias> -> <dockshim executable>` (default `bin_dir`: `.dockshim/bin`).
- `basename(argv[0])` is the alias name, which is also the command run in the container. When the name is `dockshim`, the manager CLI runs instead.
- `install` removes stale symlinks: those pointing to the current executable or to any file named `dockshim` (moved binaries, dangling links). It never touches other files.
- When a shim is invoked but its alias is gone from the config, dockshim exits 127:
  - If stdin and stderr are terminals, it first offers to delete the shim. The prompt goes through `/dev/tty`.
  - Only shims inside the project's `bin_dir` are ever offered for deletion.
- A shim outside `bin_dir` (e.g. after `bin_dir` changed) still works if its alias resolves, with a warning.

## Why
- Real files on disk let IDEs point at them. Upgrading dockshim needs no regeneration. No shell wrapper is needed.
