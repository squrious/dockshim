# 12. Project root and config discovery

Supersedes ADR 3.

## Context
Shells and IDEs launch shims, IDEs often from an unrelated working directory.

## Decision
- One config file, at the project root. Its directory is the project root.
- A shim belongs to the project whose generated directory holds it, and uses that project's config only. Nothing above the project is looked at.
- Aliases only run through their shims. Manager commands walk up from the working directory.
- No flag or environment variable overrides the location.
- dockshim owns a directory at the root that only holds generated files, and git-ignores itself. Only the config is committed.

## Why
- A shim's location names its project whatever the caller's working directory, and whether the project was moved or copied.
- Without overrides, the shell, the IDE and scripts always resolve the same config.
- Generated files never mix with user files: users never edit a gitignore, and a clone only needs `install`.

## Rejected
- Running an alias by name from the working directory: it duplicates the shims. Scripts call the command itself, IDEs need a path, and shells reach the shims through PATH.
- A config path written into the shims: it breaks when the project moves, and a copy would silently drive the original project's containers.
- Walking up from the shim: a project whose config is missing would run another project's aliases.
