# Guidelines

Never commit unless explicitly asked.

Don't be verbose in code. Only comment what's not obvious. 

Write structural decisions in `docs/adr`. Be concise here too.

# Project

`dockshim`: a Go CLI that runs commands in Docker containers as if they were on the host. Aliases are symlinks to the binary, and the alias name comes from argv[0]. The config is `.dockshim.yaml` or `.dockshim/config.yaml`. See `README.md` for usage and `docs/adr` for the design.

## Toolchain

Everything goes through the project-local `mise.toml`. Never modify the global mise config (no `mise use -g`).

- `mise install`: Go and golangci-lint
- `mise run build`: `bin/dockshim`
- `mise run test`: unit tests and CLI scripts, no docker needed
- `mise run test-integration`: real docker, pulls/uses `alpine:latest`
- `mise run lint`: go vet (with and without the integration tag) and golangci-lint

Before finishing a change, run lint and test, and also test-integration when docker behaviour changed.

## Layout

- `cmd/dockshim`: `main` only. Also holds the CLI end-to-end tests.
- `internal/cli`: argv[0] dispatch, alias mode (`alias.go`), cobra manager commands (`manager.go`). All I/O goes through `cli.Env`, so it can be injected in tests.
- `internal/config`: discovery (`discover.go`), schema, strict parse and validation, and resolution (global merged into each alias, absolute real paths, `user: host`).
- `internal/envfilter`: built-in denylist and forwarding rules.
- `internal/pathmap`: host→container path translation (longest prefix).
- `internal/docker`: the `Runner` interface (os/exec) and the `Target` implementations `Compose` and `Container`.
- `internal/execplan`: `Build` turns an alias and its args into a `Plan`. `Execute` handles auto-start, the single retry, and the `PreRun`/`PostRun` steps.
- `internal/shim`: installs, prunes and locates symlinks.

## Tests

- Unit tests are table-driven, next to the code. Fakes implement `docker.Runner` / `docker.Target`.
- CLI end-to-end tests use testscript: `cmd/dockshim/testdata/script/*.txtar`.
  - `TestMain` builds the real binary, because shims dispatch on argv[0].
  - A fake `docker` shell script (`fakeDocker` in `main_test.go`) logs calls to `$FAKE_DOCKER_STATE/calls`, emulates running state, and echoes the exec flags and env it receives.
  - Every new user-facing behaviour gets a `.txtar` scenario.
  - Inside the fake, call binaries by absolute path: shims in PATH would shadow them.
- Interactive paths (the stale-shim prompt) can't run under testscript, which has no tty. Unit-test them with an injected `Prompter`.
- Integration tests are in `cmd/dockshim/integration_test.go` (`//go:build integration`). They cover a plain container and a compose project, and clean up with `t.Cleanup`.

## Planned work (see ADR 0005)

- **Path translation for args outside the project:** implement an `execplan.ArgTransformer` that registers `PreRun` (copy into `/tmp/...` in the container) and `PostRun` (cleanup) steps. Plan how a retry should replay `PreRun`.
- **Completion forwarding:** a `dockshim completion <shell>` generator plus a hidden `complete-alias` command. The name `__complete` is taken by cobra.
