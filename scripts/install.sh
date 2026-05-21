#!/usr/bin/env bash
set -euo pipefail

REPO="JimberSoftware/vex"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/vexd.service"

VERSION=""
FORCE_SERVICE=false

usage() {
    echo "Usage: install.sh [-v VERSION] [--force-service]"
    echo ""
    echo "Install or update vex and vexd from GitHub Releases."
    echo ""
    echo "Options:"
    echo "  -v VERSION        Install a specific version (e.g. v1.2.3)"
    echo "  --force-service   Overwrite existing systemd service file"
    echo "  -h, --help        Show this help"
    exit 0
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -v) VERSION="$2"; shift 2 ;;
        --force-service) FORCE_SERVICE=true; shift ;;
        -h|--help) usage ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

if [[ $EUID -ne 0 ]]; then
    echo "Error: this script must be run as root"
    exit 1
fi

MISSING=""
for cmd in curl tar sha256sum; do
    if ! command -v "$cmd" &>/dev/null; then
        MISSING="$MISSING $cmd"
    fi
done
if [[ -n "$MISSING" ]]; then
    echo "Error: missing required tools:$MISSING"
    exit 1
fi

ARCH=$(uname -m)
case "$ARCH" in
    x86_64) GOARCH="amd64" ;;
    *) echo "Error: unsupported architecture: $ARCH"; exit 1 ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [[ "$OS" != "linux" ]]; then
    echo "Error: unsupported OS: $OS"
    exit 1
fi

if [[ -z "$VERSION" ]]; then
    echo "Fetching latest release..."
    VERSION=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
    if [[ -z "$VERSION" ]]; then
        echo "Error: could not determine latest version"
        exit 1
    fi
fi

echo "Target version: $VERSION"

CURRENT_VEX=""
CURRENT_VEXD=""
if command -v vex &>/dev/null; then
    CURRENT_VEX=$(vex --version 2>/dev/null || true)
fi
if command -v vexd &>/dev/null; then
    CURRENT_VEXD=$(vexd --version 2>/dev/null || true)
fi

CLEAN_VERSION="${VERSION#v}"
if [[ "$CURRENT_VEX" == *"$CLEAN_VERSION"* ]] && [[ "$CURRENT_VEXD" == *"$CLEAN_VERSION"* ]]; then
    echo "Already at version $VERSION — nothing to do"
    exit 0
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
VEX_ARCHIVE="vex_${OS}_${GOARCH}.tar.gz"
VEXD_ARCHIVE="vexd_${OS}_${GOARCH}.tar.gz"

echo "Downloading..."
curl -sSfL -o "$TMPDIR/$VEX_ARCHIVE" "$BASE_URL/$VEX_ARCHIVE"
curl -sSfL -o "$TMPDIR/$VEXD_ARCHIVE" "$BASE_URL/$VEXD_ARCHIVE"
curl -sSfL -o "$TMPDIR/checksums.txt" "$BASE_URL/checksums.txt"

echo "Verifying checksums..."
cd "$TMPDIR"
sha256sum --check --ignore-missing checksums.txt
cd - >/dev/null

echo "Extracting..."
tar xzf "$TMPDIR/$VEX_ARCHIVE" -C "$TMPDIR" vex
tar xzf "$TMPDIR/$VEXD_ARCHIVE" -C "$TMPDIR" vexd

if systemctl is-active --quiet vexd 2>/dev/null; then
    echo "Stopping vexd..."
    systemctl stop vexd
fi

echo "Installing binaries to $INSTALL_DIR..."
mv -f "$TMPDIR/vex" "$INSTALL_DIR/vex"
mv -f "$TMPDIR/vexd" "$INSTALL_DIR/vexd"
chmod +x "$INSTALL_DIR/vex" "$INSTALL_DIR/vexd"

if [[ ! -f "$SERVICE_FILE" ]] || [[ "$FORCE_SERVICE" == true ]]; then
    echo "Installing systemd service..."
    cat > "$SERVICE_FILE" << 'EOF'
[Unit]
Description=vexd hypervisor daemon
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vexd --listen 127.0.0.1:8080
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
fi

systemctl daemon-reload
systemctl enable --now vexd

echo ""
echo "Installed:"
echo "  vex:  $($INSTALL_DIR/vex --version)"
echo "  vexd: $($INSTALL_DIR/vexd --version)"
