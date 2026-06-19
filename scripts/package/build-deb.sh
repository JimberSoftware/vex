#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"

VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")}"
ARCH="${ARCH:-amd64}"

usage() {
    echo "Usage: build-deb.sh [-v VERSION] [-a ARCH]"
    echo ""
    echo "Build a .deb package for the vex guest agent."
    echo ""
    echo "Options:"
    echo "  -v VERSION   Package version (default: git describe)"
    echo "  -a ARCH      Architecture: amd64 or arm64 (default: amd64)"
    echo "  -h, --help   Show this help"
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -v) VERSION="$2"; shift 2 ;;
        -a) ARCH="$2"; shift 2 ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
    echo "Error: ARCH must be amd64 or arm64"
    exit 1
fi

CLEAN_VERSION="${VERSION#v}"
PACKAGE_NAME="vex-agent_${CLEAN_VERSION}_${ARCH}.deb"
BUILD_DIR="$DIST_DIR/deb-${ARCH}"

echo "==> Building vex-agent .deb package"
echo "    Version: $CLEAN_VERSION"
echo "    Architecture: $ARCH"

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/DEBIAN"
mkdir -p "$BUILD_DIR/usr/bin"
mkdir -p "$BUILD_DIR/lib/systemd/system"

echo "==> Building binary..."
cd "$ROOT_DIR"
GOOS=linux GOARCH="$ARCH" CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X github.com/jimbersoftware/vex/internal/version.Version=${CLEAN_VERSION}" \
    -o "$BUILD_DIR/usr/bin/vex-agent" \
    ./cmd/vex-agent/

echo "==> Creating control file..."
cat > "$BUILD_DIR/DEBIAN/control" << EOF
Package: vex-agent
Version: $CLEAN_VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: JimberSoftware
Description: Vex guest agent for VM communication over vsock
 The vex-agent listens on a vsock socket inside a guest VM and
 executes commands received from the host-side vex or vexd.
 .
 Supports Linux and Windows guests. This package includes a
 systemd service that starts the agent automatically on boot.
EOF

echo "==> Creating maintainer scripts..."

cat > "$BUILD_DIR/DEBIAN/postinst" << 'SCRIPT'
#!/bin/sh
set -e

case "$1" in
    configure)
        systemctl daemon-reload || true
        systemctl enable vex-agent || true
        systemctl start vex-agent || true
        ;;
    abort-upgrade|abort-remove|abort-deconfigure)
        ;;
    *)
        ;;
esac
SCRIPT

cat > "$BUILD_DIR/DEBIAN/prerm" << 'SCRIPT'
#!/bin/sh
set -e

case "$1" in
    remove|upgrade|deconfigure)
        systemctl stop vex-agent 2>/dev/null || true
        systemctl disable vex-agent 2>/dev/null || true
        ;;
    failed-upgrade)
        ;;
    *)
        ;;
esac
SCRIPT

chmod 0755 "$BUILD_DIR/DEBIAN/postinst" "$BUILD_DIR/DEBIAN/prerm"

echo "==> Copying systemd unit..."
cp "$ROOT_DIR/init/vex-agent.service" "$BUILD_DIR/lib/systemd/system/vex-agent.service"

echo "==> Building .deb..."
mkdir -p "$DIST_DIR"
dpkg-deb --root-owner-group --build "$BUILD_DIR" "$DIST_DIR/$PACKAGE_NAME"

echo ""
echo "==> Done: $DIST_DIR/$PACKAGE_NAME"
