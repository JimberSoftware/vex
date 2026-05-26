<div align="center">
  <img src="image.png" alt="Kwebbel" width="400"/>
</div>

Vex is a lightweight host-to-guest VM communication tool over [vsock](https://man7.org/linux/man-pages/man7/vsock.7.html). It provides a CLI ( `vex` ), an HTTP daemon ( `vexd` ), a golang embedable HTTP client ( `api/client`) and a cross-platform guest agent ( `vex-agent` ) for reliable command execution inside VMs

<details>
<summary><strong>Why Vex?</strong></summary>

We need to run end-to-end automation tests against ephemeral VMs spawned from Proxmox templates: installing software, running commands, and verifying integrations across different operating systems. QEMU Guest Agent worked initially, but as we scaled up (more VMs, more concurrent commands) we kept hitting timeout errors that no amount of tuning could fix.

Research led us to vsock as a faster, more stable transport. However, `qemu-ga` doesn't support binding to vsock on Windows guests, which was a dealbreaker for our multi-OS test matrix. Rather than maintaining a fork of `qemu-ga` or relying on another opaque tool where failures are hard to diagnose, we built Vex: a minimal, purpose-built tool that does one thing well — reliable host↔guest communication over vsock.

The HTTP daemon ( `vexd` ) exists so our test orchestrator can run on a separate machine and drive everything through a simple REST API.

</details>

## Install

### Quick install (Linux host)

Install or update `vex` , `vexd` , and the systemd service in one command:

```bash
curl -sSfL https://raw.githubusercontent.com/JimberSoftware/vex/main/scripts/install.sh | sudo bash
```

Pin a specific version:

```bash
curl -sSfL https://raw.githubusercontent.com/JimberSoftware/vex/main/scripts/install.sh | sudo bash -s -- -v v1.2.3
```

### Manual install

Download binaries directly from the [GitHub Releases](https://github.com/JimberSoftware/vex/releases) page.

```bash
# vex CLI client
curl -fsSL https://github.com/JimberSoftware/vex/releases/latest/download/vex_linux_amd64.tar.gz | tar xz
sudo mv vex /usr/local/bin/

# vexd HTTP daemon
curl -fsSL https://github.com/JimberSoftware/vex/releases/latest/download/vexd_linux_amd64.tar.gz | tar xz
sudo mv vexd /usr/local/bin/

# vex-agent (Linux guest)
curl -fsSL https://github.com/JimberSoftware/vex/releases/latest/download/vex-agent_linux_amd64.tar.gz | tar xz
sudo mv vex-agent /usr/local/bin/
```

### Windows (guest VM)

Download the appropriate archive for your architecture:

- **amd64**: [`vex-agent_windows_amd64.zip`](https://github.com/JimberSoftware/vex/releases/latest/download/vex-agent_windows_amd64.zip)
- **arm64**: [`vex-agent_windows_arm64.zip`](https://github.com/JimberSoftware/vex/releases/latest/download/vex-agent_windows_arm64.zip)

Extract and place `vex-agent.exe` somewhere on your `PATH` .

### Verify

```bash
vex --version
vex-agent --version
vexd --version
```

## Proxmox VE setup

Proxmox VMs need a vsock device before vex can communicate with the guest agent.

1. Stop the VM (or template).
2. SSH into the Proxmox node and edit the VM config:

```bash
nano /etc/pve/qemu-server/<VMID>.conf
```

3. Add the following line:

```
args: -device vhost-vsock-pci,guest-cid=<CID>
```

Replace `<CID>` with a unique number ≥ 3 for each VM (e.g. use `VMID` ).

4. Start the VM.

If the VM already has an `args:` line, append the vsock device to it:

```
args: <existing args> -device vhost-vsock-pci,guest-cid=<CID>
```

VMs cloned from a template inherit the vsock device, but each clone must have a unique CID so make sure to update the `args:` line for each clone.

## vex-agent

Guest-side agent that listens for incoming vsock connections.

### Build

```bash
go build ./cmd/vex-agent/
```

### Run

```bash
vex-agent [--cid <uint32>] [--port <uint32>]
```

| Flag     | Default      | Description                                                            |
| -------- | ------------ | ---------------------------------------------------------------------- |
| `--port` | `1024`       | vsock port to listen on                                                |
| `--cid`  | `4294967295` | Context ID to bind ( `4294967295` = `VMADDR_CID_ANY` , binds all CIDs) |

### Local loopback testing (Linux)

```bash
sudo modprobe vsock_loopback
vex-agent --cid 1 --port 1024
```

In a second terminal:

```bash
socat - VSOCK-CONNECT:1:1024
```

Shut down with `Ctrl+C` .
