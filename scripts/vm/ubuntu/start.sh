#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE="$SCRIPT_DIR/ubuntu-24.04.img"
DISK="$SCRIPT_DIR/ubuntu-disk.qcow2"
SEED_ISO="$SCRIPT_DIR/seed.iso"
MONITOR="$SCRIPT_DIR/monitor.sock"
IMAGE_URL="https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img"

# Download base image once
if [ ! -f "$IMAGE" ]; then
  echo "Downloading Ubuntu 24.04 cloud image..."
  curl -L -o "$IMAGE" "$IMAGE_URL"
fi

# Create a writable overlay disk from the base image
if [ ! -f "$DISK" ]; then
  qemu-img create -f qcow2 -b "$IMAGE" -F qcow2 "$DISK" 20G
fi

# Find SSH public key
SSH_KEY=""
for key_file in ~/.ssh/id_ed25519.pub ~/.ssh/id_rsa.pub ~/.ssh/id_ecdsa.pub; do
  if [ -f "$key_file" ]; then
    SSH_KEY="$(cat "$key_file")"
    break
  fi
done
if [ -z "$SSH_KEY" ]; then
  echo "ERROR: No SSH public key found in ~/.ssh/" >&2
  exit 1
fi

# Generate seed ISO with injected SSH key
TMPDIR_INIT="$(mktemp -d)"
trap 'rm -rf "$TMPDIR_INIT"' EXIT
cp "$SCRIPT_DIR/cloud-init/meta-data" "$TMPDIR_INIT/meta-data"
sed "s|__SSH_PUBKEY__|$SSH_KEY|g" "$SCRIPT_DIR/cloud-init/user-data" > "$TMPDIR_INIT/user-data"
cloud-localds "$SEED_ISO" "$TMPDIR_INIT/user-data" "$TMPDIR_INIT/meta-data"

echo "Ubuntu VM starting (CID=3, SSH on localhost:2222)..."
echo "Stop with Ctrl-C or: make vm-ubuntu-stop (from another terminal)"

exec qemu-system-x86_64 \
  -enable-kvm \
  -m 2048 -smp 2 \
  -drive file="$DISK",format=qcow2 \
  -drive file="$SEED_ISO",format=raw,media=cdrom \
  -device vhost-vsock-pci,guest-cid=3 \
  -netdev user,id=net0,hostfwd=tcp::2222-:22 \
  -device virtio-net-pci,netdev=net0 \
  -nographic \
  -monitor unix:"$MONITOR",server,nowait
