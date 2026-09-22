# 2. Alias entry points: symlinks or wrapper scripts

Superseded by ADR 10.

## Decision
- `dockshim install` creates one entry point per alias in `bin_dir`, in the mode `shim_mode` selects:
  - `symlink` (default): a link to the dockshim executable, where `basename(argv[0])` gives the alias. When that name is `dockshim`, the manager CLI runs.
  - `wrapper`: a `/bin/sh` script running `exec <dockshim> run --shim "$0" <alias> "$@"`.
- Both modes take the same code path: `--shim` is the hidden equivalent of `argv[0]`.
- `install` prunes stale entry points it owns, and only those: symlinks to a file named `dockshim`, and scripts that carry the wrapper marker comment.
- A shim whose alias is gone exits 127. On a terminal it first offers, through `/dev/tty`, to delete it, but only inside the project's `bin_dir`.

## Why
- Real files on disk let IDEs point at them. With symlinks, upgrading dockshim needs no regeneration.
- `exec -a` is not POSIX, so a wrapper passes its own path explicitly.

## Consequences
- The integration suite runs in both modes (`eachShimMode`).
- A wrapper embeds the dockshim path: after moving the binary, run `install` again.
