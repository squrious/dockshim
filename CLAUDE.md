# Guidelines

Never commit unless explicitly asked.

Don't be verbose in code. Only comment what's not obvious. 

Write structural decisions in `docs/adr`. Be concise here too.

# Project

`dockshim`: a Go CLI that runs commands in Docker containers as if they were on the host. Each alias is an entry point in `bin_dir`: a symlink to the binary (the alias name comes from argv[0]) or a `/bin/sh` wrapper calling `dockshim run --shim`, per `shim_mode`. The config is `.dockshim.yaml` or `.dockshim/config.yaml`. See `README.md` for usage and `docs/adr` for the design.

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
- `internal/cli`: argv[0] dispatch, alias mode (`alias.go`), cobra manager commands (`manager.go`), the `config` summary (`summary.go`), `init` (`init.go`) with its embedded template (`init.yaml`; keep it valid and in sync with the schema). I/O goes through `cli.Env`, so it can be injected in tests, including the environment config values are interpolated from. Deliberate exceptions read the machine itself: `hostpath.Detect`, `user: host` (`os.Getuid`), and `shim.Locate` (the real `PATH`).
- `internal/config`: discovery (`discover.go`), schema, strict parse, env interpolation of values (`interpolate.go`, ADR 0006), validation, and resolution (global merged into each alias, absolute real paths, `user: host`). `Parse` and `Load` take a `LookupFunc` (`LookupEnviron` builds one from `NAME=value` entries): pass a fake in tests, never rely on the real env.
- `internal/envfilter`: built-in denylist and forwarding rules.
- `internal/pathmap`: host→container path translation (longest prefix).
- `internal/hostpath`: what a host path means on this machine — which directories may be copied from (system temp dirs plus `allow`, see ADR 0008), and, under WSL, Windows paths read from `/proc/self/mountinfo` and `$WSL_DISTRO_NAME`. `Real` is the symlink-resolved form every package compares paths in. Everything it reads from the machine comes through `Env`, so tests inject it.
- `internal/docker`: the `Runner` interface (os/exec) and the `Target` interface, implemented by `Compose` (the only target, ADR 0009) and faked in tests.
- `internal/execplan`: `Run` starts the service if needed, infers path mappings from its bind mounts when the alias has none (`infer.go`, ADR 0009), then calls `Build`, which turns an alias and its args into a `Plan`, and `Execute`, which runs `PreRun`, the command once (no retry) and `PostRun`, warning when the service stopped under a failing command. `translate.go` implements path translation (ADRs 0007 and 0008): arguments are host paths, so mapped ones are rewritten and files in an allowed directory are copied with a tar stream through `docker cp`. Directories are never copied.
- `internal/shim`: creates, prunes and locates the alias entry points, in both modes (symlink, or `/bin/sh` wrapper script calling `dockshim run --shim`). See ADR 0002.

## Tests

- Unit tests are table-driven, next to the code. Fakes implement `docker.Runner` / `docker.Target`.
- CLI end-to-end tests use testscript: `cmd/dockshim/testdata/script/*.txtar`.
  - `TestMain` builds the real binary, because shims dispatch on argv[0].
  - A fake `docker` shell script (`fakeDocker` in `main_test.go`) logs calls to `$FAKE_DOCKER_STATE/calls`, emulates running state, echoes the exec flags and env it receives, saves the environment of `up` in `$FAKE_DOCKER_STATE/up-env`, and answers `inspect` with `$FAKE_DOCKER_STATE/mounts`. `cp` extracts into `$FAKE_DOCKER_STATE/cp`, which stands for the container's `/tmp`.
  - Every new user-facing behaviour gets a `.txtar` scenario.
  - Inside the fake, call binaries by absolute path: shims in PATH would shadow them.
- Interactive paths (the stale-shim prompt) can't run under testscript, which has no tty. Unit-test them with an injected `Prompter`.
- Integration tests are in `cmd/dockshim/integration_test.go` (`//go:build integration`). They cover a compose project, with inferred and explicit path mappings, and clean up with `t.Cleanup`. `eachShimMode` runs the whole suite in both shim modes: add scenarios to `commonChecks` so every combination stays covered.

## Planned work (see ADRs 0005, 0007, 0008 and 0009)

- **Optional copy-back** for translated paths: copies are currently one-way.
- **Optional directory copying**, if a need appears: it needs its own rules for links, special files and size.
- **Completion forwarding:** a `dockshim completion <shell>` generator plus a hidden `complete-alias` command. The name `__complete` is taken by cobra.
