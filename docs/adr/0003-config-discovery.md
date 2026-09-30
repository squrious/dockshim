# 3. Project layout and config discovery

## Decision
- The config is `.dockshim.yaml` at the project root (`.dockshim.yml` also works, both at once is an error). The directory that holds it is the project root.
- `.dockshim/` belongs to dockshim and only holds generated files. `install` writes the shims to `.dockshim/bin`, and a `.dockshim/.gitignore` of `*` unless one exists.
- Alias mode walks up from the shim's own directory first, then from cwd. The shim passes its own path.
- Manager mode walks up from cwd.
- dockshim reads no environment variables of its own.
- Paths are resolved against the real (symlink-free) root.
- Decoding is strict. Every validation problem is reported at once, each with its YAML path.

## Why
- IDEs launch shims from arbitrary directories. Anchoring on the shim's location finds the right project anyway.
- One config location, next to `compose.yaml`, and one directory of generated files that ignores itself: users never edit a gitignore.
- `install` writes the `.gitignore`, not `init`: it is never committed, and clones or new worktrees only run `install`.
- The shims sit in `bin/`, not in `.dockshim/` directly, so that other generated files never land in PATH.

## Consequences
The workdir is cwd translated through the longest matching path mapping (explicit or inferred, ADR 8). When none matches, no `--workdir` is passed.
