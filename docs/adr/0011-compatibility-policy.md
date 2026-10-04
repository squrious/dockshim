# 11. Compatibility between versions

Supersedes ADR 9.

## Decision
- **Shims are generated cache**, never committed. `install` owns their directory: it makes it hold exactly one script per alias, rewriting any script that differs from what the running version writes.
- **The runtime contract is `dockshim run --shim <path> <alias> [args]`.** A shim written by one version may run with another, so the contract stays stable in both directions.
- **Alias mode prints nothing but errors**, since IDEs and scripts parse its output. On a terminal, it also warns when the shims differ from the config: a human reads it there. Other notices and deprecations only appear in manager commands.
- **Config deprecations**: a deprecated key keeps working, with a warning, until the next breaking release. Before 1.0, a change may break directly.

## Why
- A directory dockshim owns (ADR 3) needs no way to tell its files from the user's, and rewriting by content needs no version: `install` migrates any shim.
- What the binary sees of a shim is its arguments, so they are the contract.

## Rejected
- A marker identifying dockshim's scripts: it only matters if the directory holds user files, and it turns its wording into a protocol.
- A format number in the script: the binary never reads it. A future format would be told apart by the arguments it passes, and an unmarked shim already identifies the current one.

## Consequences
Changing the shim script only requires keeping `run --shim` compatible.
