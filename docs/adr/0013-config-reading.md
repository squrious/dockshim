# 13. Reading the config

## Context
Shims run at any time, from shells and IDEs whose environments differ, and the config interpolates the environment (ADR 6).

## Decision
- Every call of a shim reads the whole config as it is at that moment, interpolated with the caller's environment.
- Every command that reads the config checks the structure of the whole file.
- `install` writes nothing that depends on the environment: it reads only what shapes the shims, without interpolating.

## Why
- Compose reads its own file at each call: reading dockshim's at the same moment keeps both in step after an edit, a pull or a branch switch.
- The environment is how other tools configure a project: changing it never requires an install.
- One rule for every shim call: it needs the whole file valid in its environment.
- A shim script holds nothing but its alias name, so install needs nothing else.

## Rejected
- Freezing the config into the shims at install, as a snapshot, as command-line flags compiled into the script, or as the alias's part of the file passed by the shim: changes go unnoticed until the next install, dockshim drifts from compose, and the shim contract (ADR 11) grows with every setting.
- Limiting a shim call to the parts of the config it uses: compose couples every call to its whole file anyway, and anchors shared between aliases make the parts hard to separate.
