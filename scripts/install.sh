#!/bin/sh
# Installs dockshim from its GitHub releases: https://github.com/squrious/dockshim
#
#   curl -fsSL https://github.com/squrious/dockshim/releases/latest/download/install.sh | sh
#
# Environment:
#   DOCKSHIM_VERSION      version to install, e.g. v0.1.0 (default: latest release)
#   DOCKSHIM_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#
# Everything runs from main, called on the last line: a truncated download runs nothing.

set -eu

REPO_URL=https://github.com/squrious/dockshim

log() { printf 'dockshim-install: %s\n' "$*" >&2; }
fail() { log "error: $*"; exit 1; }
has() { command -v "$1" >/dev/null 2>&1; }

detect_platform() {
  case $(uname -s) in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail "unsupported OS: $(uname -s) (linux and darwin are supported)" ;;
  esac
  case $(uname -m) in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) fail "unsupported architecture: $(uname -m) (amd64 and arm64 are supported)" ;;
  esac
  # A shell running under Rosetta reports x86_64 on Apple Silicon.
  if [ "$os" = darwin ] && [ "$arch" = amd64 ] && [ "$(sysctl -n hw.optional.arm64 2>/dev/null)" = 1 ]; then
    arch=arm64
  fi
}

check_tools() {
  for t in tar gzip; do
    has "$t" || fail "$t is required"
  done
}

detect_downloader() {
  if has curl; then
    downloader=curl
  elif has wget; then
    downloader=wget
  else
    fail "curl or wget is required"
  fi
}

# download URL FILE
download() {
  if [ "$downloader" = curl ]; then
    curl -fsSL -o "$2" "$1"
  else
    wget -q -O "$2" "$1"
  fi
}

# /releases/latest redirects to /releases/tag/<tag>: no API call, so no rate limit.
latest_tag() {
  if [ "$downloader" = curl ]; then
    url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$REPO_URL/releases/latest")
  else
    url=$(wget -S --spider "$REPO_URL/releases/latest" 2>&1 | sed -n 's/^ *Location: *\([^ ]*\).*/\1/p' | tail -n 1 | tr -d '\r')
  fi
  case $url in
    */releases/tag/*) printf '%s\n' "${url##*/}" ;;
    *) fail "could not resolve the latest release from $REPO_URL/releases/latest" ;;
  esac
}

sha256() {
  if has sha256sum; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif has shasum; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  else
    fail "sha256sum or shasum is required to verify the download"
  fi
}

# Sets in_path when install_dir is in PATH, and shadow to a dockshim found earlier in PATH.
# Entries are compared without trailing slashes.
check_path() {
  in_path='' shadow=''
  old_ifs=$IFS
  IFS=:
  set -f
  for d in $PATH; do
    [ -n "$d" ] || continue
    d=$(strip_slashes "$d")
    if [ "$d" = "$install_dir" ]; then
      in_path=1
      break
    fi
    if [ -z "$shadow" ] && [ -x "$d/dockshim" ]; then
      shadow=$d/dockshim
    fi
  done
  set +f
  IFS=$old_ifs
}

strip_slashes() {
  s=$(printf '%s' "$1" | sed 's:/*$::')
  printf '%s\n' "${s:-/}"
}

cleanup() {
  rm -rf "$tmp"
  if [ -n "$staged" ]; then rm -f "$staged"; fi
}

main() {
  detect_platform
  detect_downloader
  check_tools

  if [ -n "${DOCKSHIM_INSTALL_DIR:-}" ]; then
    install_dir=$DOCKSHIM_INSTALL_DIR
  elif [ -n "${HOME:-}" ]; then
    install_dir=$HOME/.local/bin
  else
    fail "HOME is not set: set DOCKSHIM_INSTALL_DIR"
  fi
  install_dir=$(strip_slashes "$install_dir")

  if [ -n "${DOCKSHIM_VERSION:-}" ]; then
    tag=v${DOCKSHIM_VERSION#v}
  else
    tag=$(latest_tag)
  fi
  version=${tag#v}
  asset=dockshim_${version}_${os}_${arch}.tar.gz
  base=$REPO_URL/releases/download/$tag

  staged=
  tmp=$(mktemp -d)
  trap cleanup EXIT
  trap 'exit 1' INT TERM

  log "downloading dockshim $tag ($os/$arch)"
  download "$base/$asset" "$tmp/$asset" ||
    fail "could not download $base/$asset: check that $tag exists. A release published minutes ago may still be uploading its files."
  download "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download $base/checksums.txt"

  expected=$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/checksums.txt")
  [ -n "$expected" ] || fail "$asset is not listed in checksums.txt"
  actual=$(sha256 "$tmp/$asset")
  [ "$actual" = "$expected" ] || fail "checksum mismatch for $asset: expected $expected, got $actual"

  tar -xzf "$tmp/$asset" -C "$tmp" dockshim

  if ! mkdir -p "$install_dir" 2>/dev/null || [ ! -w "$install_dir" ]; then
    fail "$install_dir is not writable: set DOCKSHIM_INSTALL_DIR to another directory"
  fi
  # Rename from the same directory: atomic, so running aliases keep the old binary.
  staged=$(mktemp "$install_dir/.dockshim.XXXXXX")
  cp "$tmp/dockshim" "$staged"
  chmod 755 "$staged"
  mv -f "$staged" "$install_dir/dockshim"
  staged=

  log "installed $("$install_dir/dockshim" version) to $install_dir/dockshim"

  check_path
  if [ -z "$in_path" ]; then
    log "warning: $install_dir is not in PATH. Add it in your shell profile:"
    log "  export PATH=\"$install_dir:\$PATH\""
  elif [ -n "$shadow" ]; then
    log "warning: $shadow comes first in PATH and shadows the installed binary"
  fi
}

main
