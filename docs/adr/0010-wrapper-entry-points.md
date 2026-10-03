# 10. Entry points are wrappers resolving dockshim through PATH

Supersedes ADR 2.

## Decision
- Each alias is a `/bin/sh` script running `exec dockshim run --shim "$0" <alias> "$@"`.
- `dockshim` is resolved through PATH: no binary path is stored. `dockshim` in PATH, wherever shims run, is an install requirement.
- The binary never dispatches on `argv[0]`.

## Why
A stored path broke every project's shims when an upgrade moved the binary, as mise's versioned directories do. Symlinks' advantages (no `sh` exec, the alias name in `ps`) don't outweigh this, and tools that resolve interpreter symlinks would have run the manager CLI instead of the alias.

## Consequences
Upgrading or moving dockshim never requires regenerating shims.
