agent-ubuntu:
	GOOS=linux GOARCH=amd64 go build -o vex-agent ./cmd/vex-agent/

agent-windows:
	GOOS=windows GOARCH=amd64 go build -o vex-agent.exe ./cmd/vex-agent/

agent-windows-arm:
	GOOS=windows GOARCH=arm64 go build -o vex-agent-arm.exe ./cmd/vex-agent/
