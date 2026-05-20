![VEX](image.png)

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
