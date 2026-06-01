package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jimbersoftware/vex/api"
)

func (s *Server) parseCID(r *http.Request) (uint32, error) {
	cidStr := r.PathValue("cid")
	cid, err := strconv.ParseUint(cidStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid cid %q: %w", cidStr, err)
	}
	return uint32(cid), nil
}

func (s *Server) dial(w http.ResponseWriter, r *http.Request) (AgentClient, bool) {
	cid, err := s.parseCID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	agent, err := s.Connector(cid, s.Port)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("connect to agent cid=%d: %s", cid, err))
		return nil, false
	}
	return agent, true
}

func (s *Server) setWriteDeadline(w http.ResponseWriter, timeoutSeconds uint32) {
	if timeoutSeconds == 0 {
		return
	}
	rc := http.NewResponseController(w)
	deadline := time.Now().Add(time.Duration(timeoutSeconds)*time.Second + 10*time.Second)
	_ = rc.SetWriteDeadline(deadline)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) //nolint:errchkjson
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.dial(w, r)
	if !ok {
		return
	}
	defer agent.Close()

	if err := agent.Ping(); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

func (s *Server) handleHostInfo(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.dial(w, r)
	if !ok {
		return
	}
	defer agent.Close()

	hi, err := agent.HostInfo()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, api.HostInfoResponse{
		OS:      hi.OS,
		Version: hi.Version,
		Arch:    hi.Arch,
	})
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	agent, ok := s.dial(w, r)
	if !ok {
		return
	}
	defer agent.Close()

	var req api.ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	if !req.Detach {
		s.setWriteDeadline(w, req.TimeoutSeconds)
	}

	result, err := agent.Exec(req.Command, req.Arguments, req.TimeoutSeconds, req.Username, req.Detach)
	if err != nil {
		writeJSON(w, http.StatusOK, api.ExecResponse{
			Stdout:   string(result.Stdout),
			Stderr:   string(result.Stderr),
			ExitCode: result.ExitCode,
			TimedOut: result.TimedOut,
			PID:      result.PID,
			Error:    err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, api.ExecResponse{
		Stdout:   string(result.Stdout),
		Stderr:   string(result.Stderr),
		ExitCode: result.ExitCode,
		TimedOut: result.TimedOut,
		PID:      result.PID,
	})
}
