# 3. Config location and discovery

Status: accepted (2026-09-16)

## Decision
- The config is `.dockshim.yaml` or `.dockshim/config.yaml` (`.yml` also accepted). If a directory has more than one, that is an error. The project root is the directory that owns the config.
- **Alias mode** walks up from the shim's own directory first, then from cwd. When argv[0] is a bare name, the shim is found through PATH and must resolve to dockshim.
- **Manager mode** walks up from cwd. `--config` overrides discovery.
- dockshim doesn't read any env vars of its own.
- Paths are resolved against the real (symlink-free) root, so they compare with `os.Getwd`.
- The YAML is decoded strictly (unknown keys are errors). All validation problems are reported at once, each with its YAML path.

## Why
- IDEs launch the shims from arbitrary directories. Anchoring on the shim's location finds the right project anyway.

## Consequences
- The workdir is the cwd translated through the longest matching `path_mapping`. When no mapping matches (e.g. an IDE running from `/`), no `--workdir` is passed and the container's default applies.
