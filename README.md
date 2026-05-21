![VEX](image.png)

## Install

### Quick install (Linux host)

Install or update `vex`, `vexd`, and the systemd service in one command:

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

Extract and place `vex-agent.exe` somewhere on your `PATH`.

### Verify

```bash
vex --version
vex-agent --version
vexd --version
```

## vex-agent

Guest-side agent that listens for incoming vsocket connections.

### Build

```bash
go build ./cmd/vex-agent/
```

### Run

```bash
vex-agent [--cid <uint32>] [--port <uint32>]
```

| Flag     | Default      | Description                                                          |
| -------- | ------------ | -------------------------------------------------------------------- |
| `--port` | `1024`       | vsocket port to listen on                                            |
| `--cid`  | `4294967295` | Context ID to bind (`4294967295` = `VMADDR_CID_ANY`, binds all CIDs) |

### Local loopback testing (Linux)

```bash
sudo modprobe vsock_loopback
vex-agent --cid 1 --port 1024
```

In a second terminal:

```bash
socat - VSOCK-CONNECT:1:1024
```

Shut down with `Ctrl+C`.
