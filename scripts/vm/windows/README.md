# Windows VM — One-Time Setup

## Prerequisites

- QEMU/KVM installed on the host (`qemu-kvm` package)
- `socat` installed (`socat` package)

## Step 1: Download ISOs

Place both files in this directory (`scripts/vm/windows/`):

1. **Windows 11 Evaluation ISO** (free 90-day eval):
   https://www.microsoft.com/en-us/evalcenter/evaluate-windows-11-enterprise
   Save as: `windows11.iso`

2. **VirtIO-Win ISO** (contains vsock driver):
   https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/stable-virtio/virtio-win.iso
   Save as: `virtio-win.iso`

## Step 2: Install Windows

```bash
make vm-windows-start
```

QEMU will launch. Add `-display gtk` to `start.sh` temporarily if you need a graphical display
during installation. Install Windows normally. When prompted to load storage drivers, load
`viostor` from the virtio-win ISO so QEMU's virtio disk is visible.

## Step 3: Install the VirtIO vsock Driver

After Windows boots, open the virtio-win ISO in File Explorer and run `virtio-win-gt-x64.msi`.
Select at minimum: **VirtIO Serial** and **VSOCK** (the vsock driver registers AF_VSOCK=40 with
Winsock2, which is what `vex-agent.exe` uses).

## Step 4: Enable OpenSSH Server

In Windows Settings → System → Optional Features → Add a feature → **OpenSSH Server**.

Then in an elevated PowerShell:

```powershell
Start-Service sshd
Set-Service -Name sshd -StartupType Automatic
```

Add your SSH public key:

```powershell
$key = "<paste your ~/.ssh/id_ed25519.pub content>"
New-Item -Force -Path "C:\ProgramData\ssh" -ItemType Directory
Set-Content "C:\ProgramData\ssh\administrators_authorized_keys" $key
icacls "C:\ProgramData\ssh\administrators_authorized_keys" /inheritance:r /grant "NT SERVICE\sshd:R"
```

## Step 5: Verify SSH

From the host:

```bash
ssh -p 2223 YourWindowsUser@localhost
```

## Normal Usage (after setup)

```bash
# Deploy agent binary
make agent-windows WINDOWS_USER=YourWindowsUser

# SSH in and run the agent
ssh -p 2223 YourWindowsUser@localhost
.\vex-agent.exe

# From another terminal on the host
vex --cid 4 ping
vex --cid 4 host-info
vex --cid 4 exec -- whoami
```
