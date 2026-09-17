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
- `internal/cli`: argv[0] dispatch, alias mode (`alias.go`), cobra manager commands (`manager.go`), `init` (`init.go`) with its embedded template (`init.yaml`; keep it valid and in sync with the schema). All I/O goes through `cli.Env`, so it can be injected in tests.
- `internal/config`: discovery (`discover.go`), schema, strict parse, env interpolation of values (`interpolate.go`, ADR 0006), validation, and resolution (global merged into each alias, absolute real paths, `user: host`). `Parse` takes a `LookupFunc`: pass a fake in tests, never rely on the real env.
- `internal/envfilter`: built-in denylist and forwarding rules.
- `internal/pathmap`: host→container path translation (longest prefix), and the virtual filesystems never copied.
- `internal/docker`: the `Runner` interface (os/exec) and the `Target` implementations `Compose` and `Container`.
- `internal/execplan`: `Build` turns an alias and its args into a `Plan`. `Execute` handles auto-start, the single retry (which replays `PreRun`), and the `PreRun`/`PostRun` steps. `translate.go` implements path translation (ADR 0007) as the first `ArgTransformer`: arguments are host paths, so mapped ones are rewritten and other readable files are copied with a tar stream through `docker cp`. Directories are never copied.
- `internal/shim`: installs, prunes and locates symlinks.

## Tests

- Unit tests are table-driven, next to the code. Fakes implement `docker.Runner` / `docker.Target`.
- CLI end-to-end tests use testscript: `cmd/dockshim/testdata/script/*.txtar`.
  - `TestMain` builds the real binary, because shims dispatch on argv[0].
  - A fake `docker` shell script (`fakeDocker` in `main_test.go`) logs calls to `$FAKE_DOCKER_STATE/calls`, emulates running state, and echoes the exec flags and env it receives. `cp` extracts into `$FAKE_DOCKER_STATE/cp`, which stands for the container's `/tmp`.
  - Every new user-facing behaviour gets a `.txtar` scenario.
  - Inside the fake, call binaries by absolute path: shims in PATH would shadow them.
- Interactive paths (the stale-shim prompt) can't run under testscript, which has no tty. Unit-test them with an injected `Prompter`.
- Integration tests are in `cmd/dockshim/integration_test.go` (`//go:build integration`). They cover a plain container and a compose project, and clean up with `t.Cleanup`.

## Planned work (see ADRs 0005 and 0007)

- **Optional copy-back** for translated paths: copies are currently one-way.
- **Optional directory copying**, if a need appears: it needs its own rules for links, special files and size.
- **Completion forwarding:** a `dockshim completion <shell>` generator plus a hidden `complete-alias` command. The name `__complete` is taken by cobra.
