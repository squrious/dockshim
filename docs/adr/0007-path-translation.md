# 7. Path translation

Status: accepted (2026-09-17)

## Principle
A shim must behave like a host command, so **every path in an argument is a host path**. The only exceptions are paths that cannot be copied and whose container version means the same thing.

## Decision
Each argument after the command name is checked. For `-x=value` / `--opt=value` only the value is; other options are left untouched.

| Resolved argument | Result |
|---|---|
| Under a `path_mapping`, existing or not | Rewritten to its container path |
| Under `/dev`, `/proc`, `/sys`, or a configured `exclude` | Unchanged, silently |
| Missing | Unchanged, silently (usually an output path) |
| Readable regular file within the size cap | Copied, and the argument points at the copy |
| Directory, socket, device, FIFO, unreadable, or over the cap | Unchanged, with a warning |

**Working directory and relative paths.** `-w` already puts the container in the directory matching the host cwd, so a relative argument is kept **as typed** whenever that mapping resolves it to the same file. It is rewritten when it doesn't: when the cwd is not mapped, or when the path falls under a *different* mapping (`assets: /assets/build` while `.` maps to `/app`).

**Files only.** Directories are not copied. A directory is a whole tree, with its own links, special files, permissions and size, which would spread every constraint to everything inside it. Nothing needs it today. If it comes back, it should be an explicit opt-in.

**Symlinks.** The argument is resolved first. If the target is under a mapping, the argument is rewritten and nothing is copied. Otherwise the target's content is copied under the argument's own name, since tools care about the name. A broken symlink counts as missing.

**Copying.**
- One tar stream built in Go, extracted with `docker cp --archive - <id>:/tmp`, so the container needs no binary for the copy.
- Layout: `/tmp/dockshim-<random>/<n>/<name>`, one numbered directory per file, so identical names don't collide. A path given twice is copied once.
- Ownership: a numeric user (including `host`) owns the files, with `0700` directories. For a named user, files belong to root and are made world-readable.
- A compose service is addressed through its first running container.
- `max_copy_mb` (default 100) caps the total. Over the cap, the argument is left untouched with a warning instead of failing the command: a false positive must not block a command.

**Cleanup.** `docker exec --user 0 <id> rm -rf /tmp/dockshim-<random>` runs after the command, on success or failure. A failure there is only a warning. On a retry, the copy is redone.

**Configuration.** `path_translation: {enabled, exclude, max_copy_mb}`, globally or per alias. `enabled` and `max_copy_mb` are overridden by the alias, `exclude` is appended. Enabled by default.

## Consequences / limits
- Copies are one-way. A tool that rewrites a copied file in place (a fixer, `sed -i`) loses its changes; only the warning-free copy hints at it. Copying back could become an option.
- When a path can't be copied, the argument falls back to meaning the container's path. The warning makes that visible. `exclude` turns it into the deliberate choice, for paths that should always mean the container's (e.g. `/usr/local/etc/php`).
- An argument that happens to name an existing host file is copied even if the tool doesn't treat it as a path. The size cap limits the cost, and `enabled: false` turns the feature off.
- Cleanup needs `rm` in the container.
