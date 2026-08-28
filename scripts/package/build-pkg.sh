#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
VERSION=${1:-dev}
ARCH=${2:-amd64}
OUTPUT_DIR=${OUTPUT_DIR:-"$ROOT_DIR/dist"}

if [[ $(uname -s) != "Darwin" ]]; then
    echo "Error: pkgbuild is only available on macOS" >&2
    exit 1
fi

case "$ARCH" in
    amd64|arm64) ;;
    *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

CLEAN_VERSION=${VERSION#v}
PACKAGE_ROOT=$(mktemp -d)
trap 'rm -rf "$PACKAGE_ROOT"' EXIT

mkdir -p "$PACKAGE_ROOT/usr/local/bin"
mkdir -p "$PACKAGE_ROOT/Library/LaunchDaemons"
mkdir -p "$OUTPUT_DIR"

GOOS=darwin GOARCH="$ARCH" CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X github.com/jimbersoftware/vex/internal/version.Version=${VERSION}" \
    -o "$PACKAGE_ROOT/usr/local/bin/vex-agent" \
    "$ROOT_DIR/cmd/vex-agent"

install -m 0644 \
    "$ROOT_DIR/init/io.jimber.vex-agent.plist" \
    "$PACKAGE_ROOT/Library/LaunchDaemons/io.jimber.vex-agent.plist"

pkgbuild \
    --root "$PACKAGE_ROOT" \
    --scripts "$ROOT_DIR/scripts/package/macos" \
    --identifier io.jimber.vex-agent \
    --version "$CLEAN_VERSION" \
    --install-location / \
    --ownership recommended \
    "$OUTPUT_DIR/vex-agent_${CLEAN_VERSION}_${ARCH}.pkg"
