# 3. Project root and config discovery

## Context
Shells and IDEs launch shims, IDEs often from an unrelated working directory.

## Decision
- One config file, at the project root. Its directory is the project root.
- From a shim, discovery walks up from the shim's own location, then from the working directory. Manager commands walk up from the working directory.
- No flag or environment variable overrides the location.
- dockshim owns a directory at the root that only holds generated files, and git-ignores itself. Only the config is committed.

## Why
- Anchoring on the shim finds the right project whatever the caller's working directory.
- Without overrides, the shell, the IDE and scripts always resolve the same config.
- Generated files never mix with user files: users never edit a gitignore, and a clone only needs `install`.
