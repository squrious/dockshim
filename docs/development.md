# Development

The toolchain is pinned in `mise.toml`:

```bash
mise install                 # Go, golangci-lint, goreleaser, actionlint, shellcheck
mise run build               # bin/dockshim
mise run test                # unit and CLI tests, no docker needed
mise run test-integration    # real docker, uses alpine:latest
mise run lint                # go vet, golangci-lint, goreleaser check, actionlint, shellcheck
mise run release-snapshot    # release archives into dist/, nothing published
```

## Tests

- **Unit tests** sit next to the code. Docker is faked through the `docker.Runner` and `docker.Target` interfaces.
- **CLI tests** are [testscript](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) scenarios in `cmd/dockshim/testdata/script`. They run the real binary against a fake `docker` script.
- **Integration tests** (`-tags integration`) run a compose project against a real daemon.
- **Install script**: `.github/workflows/install.yml` runs `scripts/install.sh` against the real GitHub releases on Linux and macOS (latest, pinned, unknown version, wget on Linux, shasum on macOS). It runs when the script or `.goreleaser.yaml` changes, and after each release against the published `install.sh`.

## Commits

Commits follow [Conventional Commits](https://www.conventionalcommits.org): `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `ci:`, `chore:`. A `!` or a `BREAKING CHANGE:` footer marks a breaking change.

## Releasing

Releases are automated with [release-please](https://github.com/googleapis/release-please) and [GoReleaser](https://goreleaser.com):

1. Merge conventional commits into `main`. PRs should be squash-merged with a conventional title.
2. release-please keeps a `chore(main): release X.Y.Z` PR open, with the version bump and the `CHANGELOG.md` entry.
3. Merging that PR tags `vX.Y.Z` and creates the GitHub release. GoReleaser then attaches the linux/darwin archives, their checksums and `install.sh`, and the install script is smoke-tested against the new release.

Version bumps, while below 1.0:
- `fix` bumps the patch.
- `feat` and breaking changes bump the minor.

Once the config schema and CLI are stable, a commit with a `Release-As: 1.0.0` footer makes 1.0.0. From then on, breaking changes bump the major.

Overrides:
- A `Release-As: x.y.z` footer forces the next version.
- Pushing a `vX.Y.Z` tag yourself runs GoReleaser directly, without release-please.

release-please needs "Allow GitHub Actions to create and approve pull requests", in the repository's Actions settings.
