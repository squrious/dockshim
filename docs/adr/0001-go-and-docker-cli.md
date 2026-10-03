# 1. Go, driving the docker CLI

## Decision
dockshim is written in Go, and runs the `docker` CLI rather than using the Engine SDK.

## Why
- Go gives a single static binary: nothing else to install on the host.
- The CLI already handles contexts, compose file discovery, `.env` files, TTY and stdin forwarding. Compose has no stable Go API, so the SDK would mean reimplementing all of it.

## Consequences
- `docker` must be in PATH.
- Every docker call goes through one runner interface, which tests fake.
