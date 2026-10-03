# 4. Environment forwarding

## Context
A host command sees the caller's environment, and tools rely on it.

## Decision
- The host environment is forwarded by default, minus a denylist of variables that describe the host. The config can extend or override it.
- Variables are forwarded by name only: docker reads the values from its own environment.
- Commands run as the caller's uid:gid by default.

## Why
- An allowlist would break every tool reading a variable nobody listed. Host-specific variables are a short, known list.
- Values in docker's arguments would leak secrets through `ps`.
- Files created in bind mounts then belong to the caller, as with a host command.
