# 14. Environment forwarding

Supersedes ADR 4.

## Context
A host command sees the caller's environment, and tools rely on it. Variables set by the config are meant for the container, often with container paths as values.

## Decision
- Host variables are forwarded by name, minus a denylist of variables that describe the host.
- Variables set by the config are passed by value.
- Docker runs with the caller's environment, unchanged.
- Commands run as the caller's uid:gid by default.

## Why
- An allowlist would break every tool reading a variable nobody listed. Host-specific variables are a short, known list.
- By name, a value has to be in docker's own environment: a config variable such as a home or a search path would then change docker's context, plugins and credentials.
- Files created in bind mounts belong to the caller, as with a host command.

## Rejected
- An environment file instead of arguments: `compose exec` has none, so it would mean leaving it.
- Passing by value only the names docker reads: the list is open-ended.

## Consequences
Config values show in the process arguments. A secret should be forwarded from the host by name.
