# 3. Config location and discovery

## Decision
- The config is `.dockshim.yaml` or `.dockshim/config.yaml` (`.yml` also works). More than one in a directory is an error. The directory that owns it is the project root.
- Alias mode walks up from the shim's own directory first, then from cwd. A bare `argv[0]` is resolved through PATH.
- Manager mode walks up from cwd. `--config` overrides discovery.
- dockshim reads no environment variables of its own.
- Paths are resolved against the real (symlink-free) root.
- Decoding is strict. Every validation problem is reported at once, each with its YAML path.

## Why
IDEs launch shims from arbitrary directories. Anchoring on the shim's location finds the right project anyway.

## Consequences
The workdir is cwd translated through the longest matching path mapping (explicit or inferred, ADR 8). When none matches, no `--workdir` is passed.
