# 1. Go, driving the docker CLI

## Decision
- Written in Go, with the toolchain and tasks in the project `mise.toml`.
- dockshim runs the `docker` / `docker compose` binaries instead of using the Engine SDK.

## Why
- Go builds a single static binary, and cobra and yaml.v3 are mature.
- The CLI already handles contexts, compose file discovery, `.env` files, TTY and stdin forwarding. Compose has no stable Go API.

## Consequences
- `docker` must be in PATH. Every call goes through `docker.Runner`, which tests fake.
