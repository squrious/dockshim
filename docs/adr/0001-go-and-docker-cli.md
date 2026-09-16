# 1. Go, driving the docker CLI

Status: accepted (2026-09-16)

## Decision
- Written in Go. It builds to a single static binary, and yaml.v3 and cobra are mature. The toolchain is pinned in the project `mise.toml`, and tasks are mise tasks.
- dockshim runs the `docker` / `docker compose` binaries instead of using the Engine SDK.

## Why
- The CLI already handles docker contexts, compose file discovery, `.env` files, TTY and stdin forwarding, which would be costly to reproduce with the SDK.
- The binary stays small (~5 MB). Compose has no stable Go API anyway.

## Consequences
- `docker` must be in PATH. All calls go through `docker.Runner`, which tests replace with a fake.
