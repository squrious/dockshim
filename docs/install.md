# Installation

dockshim is a single binary. Whichever method you use, `dockshim` must be in the `PATH` of whatever runs the entry points, IDEs included: they run `dockshim run --shim …` through `PATH`.

Prebuilt binaries exist for Linux and macOS, on amd64 and arm64. Under WSL, use the Linux binary.

## Install script

```bash
curl -fsSL https://github.com/squrious/dockshim/releases/latest/download/install.sh | sh
```

The script:
1. detects the OS and architecture (arm64 on Apple Silicon, even from a Rosetta shell);
2. resolves the latest release through the `releases/latest` redirect (no GitHub API call, so no rate limit), or uses `DOCKSHIM_VERSION`;
3. downloads the archive and `checksums.txt` from the release, and verifies the SHA-256 checksum;
4. atomically replaces `dockshim` in the install directory, so running aliases are not disturbed;
5. warns when the install directory isn't in `PATH`, or when another `dockshim` comes first in `PATH`. It never edits your shell profile, and never uses `sudo`.

It needs `sh`, `curl` or `wget`, `tar`, `gzip`, and `sha256sum` or `shasum`.

| Variable | Default | |
|---|---|---|
| `DOCKSHIM_VERSION` | latest release | Version to install, e.g. `v0.1.0` or `0.1.0` |
| `DOCKSHIM_INSTALL_DIR` | `~/.local/bin` | Where to put the binary |

The variables must be set for `sh`, not for `curl`:

```bash
curl -fsSL https://github.com/squrious/dockshim/releases/latest/download/install.sh | DOCKSHIM_VERSION=v0.1.0 sh
```

The script is attached to each release. To read it before running it:

```bash
curl -fsSL -o install.sh https://github.com/squrious/dockshim/releases/latest/download/install.sh
less install.sh
sh install.sh
```

**Upgrade:** run the script again. Right after a release is published, its files may take a few minutes to be attached: if the download fails, retry later. **Uninstall:** delete the binary (`rm ~/.local/bin/dockshim`).

## mise

[mise](https://mise.jdx.dev) can install dockshim from its GitHub releases:

```bash
mise use -g github:squrious/dockshim
```

Upgrade with `mise upgrade github:squrious/dockshim`.

`mise activate` sets `PATH` in interactive shells only. IDEs started from a desktop launcher don't see it: add mise's shims directory (`~/.local/share/mise/shims`) to the `PATH` they get, or configure your IDE's mise integration.

## Manual download

Download the archive for your platform and `checksums.txt` from the [releases](https://github.com/squrious/dockshim/releases), then:

```bash
sha256sum -c --ignore-missing checksums.txt   # macOS: shasum -a 256 -c --ignore-missing checksums.txt
tar -xzf dockshim_*_linux_amd64.tar.gz dockshim
mkdir -p ~/.local/bin
mv dockshim ~/.local/bin/
```

## go install

With Go installed:

```bash
go install github.com/squrious/dockshim/cmd/dockshim@latest
```

The binary goes to `$(go env GOPATH)/bin`. `dockshim version` shows the module version.
