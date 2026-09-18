# 8. Allowed copy paths, and Windows paths under WSL

Status: accepted (2026-09-17)

Amends [0007](0007-path-translation.md): the `exclude` denylist and the "any readable regular file is copied" rule are replaced by the allow list below.

## Copying is allow-listed
Copying exists for the throwaway files a tool hands a command: an IDE test runner script, an editor buffer, a generated config. A denylist was the wrong shape for that — an unbounded list to fence off an unbounded default — so the default is now closed.

A file is copied only when it sits in an **allowed directory**:
- the system temporary directories: `TMPDIR`, `/tmp`, `/var/tmp`;
- under WSL, the Windows ones seen through a drive mount: `<drive>/Windows/Temp` and `<drive>/Users/<user>/AppData/Local/Temp`;
- anything in `path_translation.allow`.

Anything else keeps its value, and so means the container's own path. That is what `exclude` used to make deliberate; it is now the default, and `exclude` is gone. Parsing is strict, so an old `exclude:` key fails with a clear error.

`allow` entries only have to be absolute, so `allow: ["/"]` is the one-line way back to the previous behaviour. Naming real directories is better, but the choice belongs to the user.

**Windows temp is matched by shape**, not read from `%TEMP%`: reading it means running `cmd.exe`, ~100 ms on every invocation. A relocated `%TEMP%` goes in `allow`. Querying it once could become an option if the shape proves too narrow.

**Warnings.** Not copying is now the ordinary outcome, so it is silent. The warnings of 0007 (directory, over the cap, unreadable, not a regular file) are kept for a path that *is* in an allowed directory, where the user meant it to be copied.

## Symlinks stay inside
A symlink is two paths: where it sits and what it points at. Both must be allowed — the target decides — so a link cannot walk a copy out of a temporary directory unnoticed.

`path_translation.follow_symlinks` (default `false`) accepts the link's own location instead, for a project that deliberately links fixtures into a temporary directory.

## Windows paths are translated
A Windows tool driving a shim in WSL passes Windows paths. PhpStorm running a test gives `C:/Users/<user>/AppData/Local/Temp/ide-phpunit.php` and `\\wsl.localhost\<distro>\home\<user>\proj\tests`. Neither is absolute to Go, so both were joined to the working directory, resolved to nothing and reached the container as typed. Both name files this distribution can reach, so both are translated first, before mapping and before the allow check:

| Written as | Becomes |
|---|---|
| `C:\x`, `C:/x` | the drive's mount point + `/x`, for a mounted drive |
| `\\wsl.localhost\<distro>\x`, `\\wsl$\<distro>\x`, or either spelled `//wsl.localhost/<distro>/x` | `/x`, for this distribution only |
| an unmounted drive, another distribution, a network share | unchanged, **with a warning** |

The warning is the exception to the silence above: a Windows path is unambiguously a path, and leaving it as typed is certain to fail.

**Detection is syntactic and strict** — `X:\`, `X:/`, a leading `\\`, or a leading `//` followed by a distribution host. An argument that merely contains backslashes, such as a PHPUnit `--filter` over namespaced class names, must not be mistaken for a path.

A UNC path may arrive with forward slashes, and one IDE invocation can use both spellings at once. The backslash form is unambiguous, but `//x/y` is also an ordinary absolute path, so the forward slash form counts only when the host names a distribution (`wsl.localhost`, `wsl$`). `//server/share` and `//etc/hostname` stay Linux paths.

**Drives are read from `/proc/self/mountinfo`** (fstype `drvfs`, or `9p`/`virtiofs` announcing `aname=drvfs`), and the distribution name from `$WSL_DISTRO_NAME`. `wslpath` would be canonical but costs a process per argument; parsing is a single file read, and takes a fixture in tests.

## Consequences / limits
- A file the tool did copy before, outside any temporary directory, now silently means the container's own path. `allow` is the fix.
- Drive-letter translation has no `.txtar` scenario: `/proc/self/mountinfo` cannot be injected across the process boundary, and a hidden environment override only for tests is not worth it. Unit tests cover it; the UNC form, which needs only `$WSL_DISTRO_NAME`, is covered end to end.
- Only the running distribution is reachable. `\\wsl.localhost\<other>\…` warns rather than guessing at a path.
