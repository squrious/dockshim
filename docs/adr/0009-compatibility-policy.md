# 9. Compatibility between versions

## Decision
- **Shims are generated cache.** They are git-ignored and recognised by their marker. `install` is the migration tool: it rewrites any shim whose content differs from what the running version would write.
- **The runtime contract is `dockshim run --shim <path> <alias> [args]`.** Shims resolve `dockshim` through PATH (ADR 10), so a shim written by one version may run with another. The contract stays stable in both directions.
- **Shims carry a header**: a format number, bumped only when the script changes, and the dockshim version that wrote them. The version is for diagnostics only: `install` ignores it when comparing, so upgrades don't rewrite every shim. Future migrations can key on the format.
- **Alias mode prints nothing but errors**, since IDEs and scripts parse its output. Deprecations and notices only appear in manager commands.
- **Config deprecations**: a deprecated key is still accepted, with a warning in manager commands, and removed at the next breaking release. Before 1.0, a change may break directly.

## Consequences
- Changing the script means bumping the format, and keeping `run --shim` compatible with the previous format.
- Outdated shims keep working until `install` runs again.
