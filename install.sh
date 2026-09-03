#!/bin/sh
#
# koa installer.
#
#   curl -fsSL https://raw.githubusercontent.com/tdanbo/koa/main/install.sh | sh
#
# Downloads the latest koa release from GitHub and installs the binary. koa
# dogfoods its own naming convention, so the asset it looks for is exactly the
# one maintainers must publish to be koa-installable:
#
#   koa-{version}-amd64-linux.tar.gz
#
# Environment:
#   KOA_REPO         owner/name to install from       (default tdanbo/koa)
#   KOA_VERSION      release tag to install           (default: latest)
#   KOA_INSTALL_DIR  where to place the binary        (default: ~/.local/bin)
#   GITHUB_TOKEN     token for private repos / higher rate limits
#   KOA_API          GitHub API root                 (default api.github.com)
#
# Windows is not covered: `curl | sh` is a Unix pattern. Use install.ps1
# instead (PRD §17).

set -eu

REPO="${KOA_REPO:-tdanbo/koa}"
VERSION="${KOA_VERSION:-}"
INSTALL_DIR="${KOA_INSTALL_DIR:-$HOME/.local/bin}"
API="${KOA_API:-https://api.github.com}/repos/$REPO"

log() { printf '%s\n' "$*" >&2; }
die() { printf 'koa: %s\n' "$*" >&2; exit 1; }

# --- preflight -------------------------------------------------------------

case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) die "macOS is not supported (see PRD §3). Build from source if you need it." ;;
  *) die "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) ARCH=amd64 ;;
  *) die "unsupported architecture: $(uname -m). koa ships amd64 binaries only." ;;
esac

if command -v curl >/dev/null 2>&1; then
  DOWNLOADER=curl
elif command -v wget >/dev/null 2>&1; then
  DOWNLOADER=wget
else
  die "neither curl nor wget is installed"
fi

command -v tar >/dev/null 2>&1 || die "tar is required to unpack the release"

# fetch writes the body of a URL to stdout, adding auth when a token is set.
fetch() {
  url="$1"
  accept="${2:-application/vnd.github+json}"
  if [ "$DOWNLOADER" = curl ]; then
    set -- -fsSL -H "Accept: $accept" -H "User-Agent: koa-install"
    [ -n "${GITHUB_TOKEN:-}" ] && set -- "$@" -H "Authorization: Bearer $GITHUB_TOKEN"
    curl "$@" "$url"
  else
    set -- -qO- --header="Accept: $accept" --header="User-Agent: koa-install"
    [ -n "${GITHUB_TOKEN:-}" ] && set -- "$@" --header="Authorization: Bearer $GITHUB_TOKEN"
    wget "$@" "$url"
  fi
}

# --- resolve the release ---------------------------------------------------

if [ -n "$VERSION" ]; then
  RELEASE_URL="$API/releases/tags/$VERSION"
else
  RELEASE_URL="$API/releases/latest"
fi

log "Fetching release metadata from $REPO…"
RELEASE_JSON=$(fetch "$RELEASE_URL") || die "could not read $RELEASE_URL"

TAG=$(printf '%s' "$RELEASE_JSON" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
[ -n "$TAG" ] || die "no release found for $REPO"

# koa's own naming convention. The version segment is whatever the tag holds,
# with a leading `v` stripped, which is how koa publishes its assets.
STRIPPED=${TAG#v}
ASSET="koa-$STRIPPED-$ARCH-$OS.tar.gz"

# Find the asset's API URL. Using the API URL rather than
# browser_download_url is what makes an install from a private repo work with
# a token. jq is used when present; otherwise the object is located by
# collapsing whitespace and splitting the payload on `{`.
if command -v jq >/dev/null 2>&1; then
  ASSET_URL=$(printf '%s' "$RELEASE_JSON" | jq -r --arg name "$ASSET" \
    '.assets[] | select(.name == $name) | .url' | head -n 1)
else
  ASSET_URL=$(
    printf '%s' "$RELEASE_JSON" \
      | tr -d '\n\r \t' \
      | tr '{' '\n' \
      | grep -F "\"name\":\"$ASSET\"" \
      | sed -n 's|.*"url":"\([^"]*releases/assets/[0-9][0-9]*\)".*|\1|p' \
      | head -n 1
  )
fi

[ -n "$ASSET_URL" ] || die "release $TAG publishes no asset named $ASSET"

# --- download and unpack ---------------------------------------------------

TMP=$(mktemp -d "${TMPDIR:-/tmp}/koa-install.XXXXXX") || die "could not create a temporary directory"
trap 'rm -rf "$TMP"' EXIT INT TERM

log "Downloading $ASSET ($TAG)…"
fetch "$ASSET_URL" "application/octet-stream" > "$TMP/$ASSET" || die "download failed"

tar -xzf "$TMP/$ASSET" -C "$TMP" || die "could not unpack $ASSET"

BINARY=$(find "$TMP" -type f -name koa -perm -u+x 2>/dev/null | head -n 1)
[ -n "$BINARY" ] || BINARY=$(find "$TMP" -type f -name koa 2>/dev/null | head -n 1)
[ -n "$BINARY" ] || die "no koa binary inside $ASSET"

# --- install ---------------------------------------------------------------

mkdir -p "$INSTALL_DIR" || die "could not create $INSTALL_DIR"
install -m 0755 "$BINARY" "$INSTALL_DIR/koa" 2>/dev/null || {
  cp "$BINARY" "$INSTALL_DIR/koa" && chmod 0755 "$INSTALL_DIR/koa"
} || die "could not write to $INSTALL_DIR — set KOA_INSTALL_DIR to a writable directory"

log ""
log "koa $TAG installed to $INSTALL_DIR/koa"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) log "Run it with: koa" ;;
  *)
    log ""
    log "$INSTALL_DIR is not on your PATH. Add this to your shell profile:"
    log ""
    log "    export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac
