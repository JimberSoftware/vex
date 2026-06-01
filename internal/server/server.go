package server

import (
	"log/slog"
	"net/http"

	"github.com/jimbersoftware/vex/client"
)

type AgentClient interface {
	Ping() error
	HostInfo() (client.HostInfo, error)
	Exec(command string, args []string, timeoutSeconds uint32, username string, detach bool) (client.ExecResult, error)
	Close() error
}

type AgentConnector func(cid, port uint32) (AgentClient, error)

func DefaultAgentConnector(cid, port uint32) (AgentClient, error) {
	return client.New(cid, port)
}

type Server struct {
	Connector AgentConnector
	Port      uint32
	Log       *slog.Logger
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /vms/{cid}/ping", s.handlePing)
	mux.HandleFunc("POST /vms/{cid}/host-info", s.handleHostInfo)
	mux.HandleFunc("POST /vms/{cid}/exec", s.handleExec)
	return mux
}
