#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DISK="$SCRIPT_DIR/windows11.qcow2"
WIN_ISO="$SCRIPT_DIR/windows11.iso"
VIRTIO_ISO="$SCRIPT_DIR/virtio-win.iso"
MONITOR="$SCRIPT_DIR/monitor.sock"

if [ ! -f "$WIN_ISO" ]; then
  echo "ERROR: $WIN_ISO not found." >&2
  echo "Download from https://www.microsoft.com/en-us/evalcenter/evaluate-windows-11-enterprise" >&2
  echo "See scripts/vm/windows/README.md for setup instructions." >&2
  exit 1
fi

if [ ! -f "$VIRTIO_ISO" ]; then
  echo "ERROR: $VIRTIO_ISO not found." >&2
  echo "Download from https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso" >&2
  exit 1
fi

# Create disk image on first run (installation target)
if [ ! -f "$DISK" ]; then
  qemu-img create -f qcow2 "$DISK" 60G
  echo "Created $DISK (60G) — QEMU will boot the Windows installer."
  echo "Install Windows, then install virtio-vsock driver from the virtio-win ISO."
  echo "Enable OpenSSH Server after install. See README.md."
fi

echo "Windows VM starting (CID=4, SSH on localhost:2223)..."
echo "Stop with Ctrl-C or: make vm-windows-stop (from another terminal)"

exec qemu-system-x86_64 \
  -enable-kvm \
  -m 4096 -smp 4 \
  -drive file="$DISK",format=qcow2 \
  -drive file="$WIN_ISO",media=cdrom,readonly=on,index=1 \
  -drive file="$VIRTIO_ISO",media=cdrom,readonly=on,index=2 \
  -device vhost-vsock-pci,guest-cid=4 \
  -netdev user,id=net0,hostfwd=tcp::2223-:22 \
  -device virtio-net-pci,netdev=net0 \
  -display vnc=:2 \
  -monitor unix:"$MONITOR",server,nowait
