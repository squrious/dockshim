# 7. Path translation

## Context
A shim must behave like a host command, so its arguments are host paths. The container only sees the host through its bind mounts. Tools, IDEs especially, also hand commands throwaway files from temporary directories.

## Decision
Every argument, or the value of `--opt=value`, is treated as a potential host path:
- Under a path mapping, it is rewritten to its container path.
- A file in an allowed directory is copied into the container for the duration of the command. Only temporary directories are allowed by default.
- Anything else is left as typed: it means the container's own path.

Under WSL, the Windows paths a Windows tool passes are converted to Linux paths first.

## Why
- Knowing which options of which tool take paths doesn't scale: inspecting every argument works for any tool.
- Mounting paths at run time would mean recreating the container.
- Copying is closed by default, since an argument that merely names a host file must not upload it. Only files are copied: a directory brings links, special files and an unbounded size.
- Leaving unknown paths alone keeps container paths and non-path arguments working.

## Consequences
- Copies are one-way: in-place edits made in the container are lost.
- An argument that happens to name a file in an allowed directory is copied.
