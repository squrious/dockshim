# 7. Path translation

## Principle
A shim must behave like a host command, so **every path in an argument is a host path**. Each argument after the command name is checked. For `-x=value` / `--opt=value` only the value is checked; other options are left alone.

## Decision
Windows paths are translated first (see below). Then:

| Resolved argument | Result |
|---|---|
| Under a path mapping, existing or not | Rewritten to its container path |
| Missing | Unchanged, silently (usually an output path) |
| Readable regular file within the size cap, in an allowed directory | Copied, and the argument points at the copy |
| Anywhere else | Unchanged, silently: it means the container's own path |
| In an allowed directory, but a directory, special, unreadable or over the cap | Unchanged, with a warning |

- **Relative paths** are kept as typed when the workdir mapping resolves them to the same file, and rewritten otherwise.
- **Allowed directories** are the system temporary ones (`TMPDIR`, `/tmp`, `/var/tmp`), the Windows ones seen through a drive mount under WSL, and `path_translation.allow`. Copying exists for the throwaway files a tool hands a command, so the default is closed.
- **Windows temp is matched by shape** (`<drive>/Windows/Temp`, `<drive>/Users/<user>/AppData/Local/Temp`), not read from `%TEMP%`: running `cmd.exe` costs ~100 ms per call.
- **Symlinks** are resolved. A target under a mapping gets rewritten. Otherwise, the target must be allowed, unless `follow_symlinks` is set, and its content is copied under the link's name.
- **Files only.** A directory brings its own links, special files and size, so directories are never copied.
- **Copying** uses one tar stream built in Go, extracted with `docker cp`, so the container needs no binary for it. Files go to `/tmp/dockshim-<random>/<n>/<name>`.
  - A numeric user owns the files. For a named user, root owns them and they are made world-readable.
  - `max_copy_mb` caps the total: a false positive must not block a command.
  - `rm -rf` runs as root after the command. A failure there is only a warning.

## Windows paths under WSL
A Windows tool driving a shim passes Windows paths, which Go sees as relative:
- `C:\x` / `C:/x` becomes the drive's mount point, read from `/proc/self/mountinfo` (`drvfs`, or `9p`/`virtiofs` with `aname=drvfs`). `wslpath` would cost a process per argument.
- `\\wsl.localhost\<distro>\x`, `\\wsl$\...`, or their `//` spelling becomes `/x`, for `$WSL_DISTRO_NAME` only.
- Anything unreachable stays as typed, with a warning: unlike other paths, it is certain to fail.

Detection is strict: `X:\`, `X:/`, a leading `\\`, or `//` followed by a distribution host. A `--filter` over namespaced PHP classes must not look like a path, and `//server/share` stays a Linux path.

## Consequences
- Copies are one-way: in-place edits (a fixer, `sed -i`) are lost.
- An argument that happens to name a file in an allowed directory is copied, even when the tool doesn't treat it as a path. The size cap limits the cost.
- Drive letters are unit tested only: `/proc/self/mountinfo` can't be injected into a `.txtar` scenario.
