# Development

The toolchain is pinned in `mise.toml`:

```bash
mise install                 # Go and golangci-lint
mise run build               # bin/dockshim
mise run test                # unit and CLI tests, no docker needed
mise run test-integration    # real docker, uses alpine:latest
mise run lint                # go vet and golangci-lint
```

## Tests

- **Unit tests** sit next to the code. Docker is faked through the `docker.Runner` and `docker.Target` interfaces.
- **CLI tests** are [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) scenarios in `cmd/dockshim/testdata/script`. They run the real binary against a fake `docker` script.
- **Integration tests** (`-tags integration`) run a compose project against a real daemon, in both shim modes.

## Commits

Commits follow [Conventional Commits](https://www.conventionalcommits.org): `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`. A `!` or a `BREAKING CHANGE:` footer marks a breaking change.
