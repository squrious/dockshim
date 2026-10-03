# 9. Compatibility between versions

## Decision
- **Shims are generated cache**, never committed. `install` is the migration tool: it rewrites any shim that differs from what the running version would write.
- **The runtime contract is `dockshim run --shim <path> <alias> [args]`.** A shim written by one version may run with another, so the contract stays stable in both directions. Shims carry a format number, for future migrations.
- **Alias mode prints nothing but errors**, since IDEs and scripts parse its output. Notices and deprecations only appear in manager commands.
- **Config deprecations**: a deprecated key keeps working, with a warning, until the next breaking release. Before 1.0, a change may break directly.

## Consequences
Changing the shim script means bumping its format, and keeping `run --shim` compatible with the previous one.
