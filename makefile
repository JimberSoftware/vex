VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")

agent-ubuntu:
	GOOS=linux GOARCH=amd64 go build -o vex-agent ./cmd/vex-agent/

agent-windows:
	GOOS=windows GOARCH=amd64 go build -o vex-agent.exe ./cmd/vex-agent/

agent-windows-arm:
	GOOS=windows GOARCH=arm64 go build -o vex-agent-arm.exe ./cmd/vex-agent/

installer: agent-windows
	@mkdir -p dist
	@cp vex-agent.exe dist/vex-agent.exe
	makensis -DVERSION=$(VERSION) -DBIN_PATH=../../dist/vex-agent.exe scripts/installer/vex-agent.nsi
	@echo "Installer built: dist/vex-agent-installer.exe"
