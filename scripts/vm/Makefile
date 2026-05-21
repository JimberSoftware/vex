WINDOWS_USER ?= User
REPO_ROOT    := $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../..)

.PHONY: vm-ubuntu-start vm-ubuntu-stop vm-windows-start vm-windows-stop agent-ubuntu agent-windows

vm-ubuntu-start:
	@bash $(dir $(lastword $(MAKEFILE_LIST)))ubuntu/start.sh

vm-ubuntu-stop:
	@bash $(dir $(lastword $(MAKEFILE_LIST)))ubuntu/stop.sh

vm-windows-start:
	@bash $(dir $(lastword $(MAKEFILE_LIST)))windows/start.sh

vm-windows-stop:
	@bash $(dir $(lastword $(MAKEFILE_LIST)))windows/stop.sh

agent-ubuntu:
	GOOS=linux GOARCH=amd64 go build -C $(REPO_ROOT) -o /tmp/vex-agent ./cmd/vex-agent/
	scp -o StrictHostKeyChecking=no -P 2222 /tmp/vex-agent ubuntu@localhost:~/vex-agent
	@echo ""
	@echo "Agent deployed. To start it:"
	@echo "  ssh -p 2222 ubuntu@localhost"
	@echo "  ./vex-agent"
	@echo ""
	@echo "Then from the host: vex --cid 3 ping"

agent-windows:
	GOOS=windows GOARCH=amd64 go build -C $(REPO_ROOT) -o /tmp/vex-agent.exe ./cmd/vex-agent/
	scp -o StrictHostKeyChecking=no -P 2223 /tmp/vex-agent.exe $(WINDOWS_USER)@localhost:"C:/Users/$(WINDOWS_USER)/vex-agent.exe"
	@echo ""
	@echo "Agent deployed. To start it:"
	@echo "  ssh -p 2223 $(WINDOWS_USER)@localhost"
	@echo "  .\\vex-agent.exe"
	@echo ""
	@echo "Then from the host: vex --cid 4 ping"
