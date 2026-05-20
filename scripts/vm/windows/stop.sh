#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MONITOR="$SCRIPT_DIR/monitor.sock"

if [ -S "$MONITOR" ]; then
  echo "system_powerdown" | socat - "UNIX-CONNECT:$MONITOR"
  echo "Sent ACPI powerdown to Windows VM"
else
  echo "Monitor socket not found — killing QEMU process for CID=4"
  pkill -f "guest-cid=4" || echo "No matching QEMU process found"
fi
