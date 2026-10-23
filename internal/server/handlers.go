package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jimbersoftware/vex/api"
)

const maxUploadSize = 2 * 1024 * 1024 * 1024

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

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if r.ContentLength < 0 {
		writeError(w, http.StatusLengthRequired, "content length is required")
		return
	}
	if r.ContentLength > maxUploadSize {
		writeError(w, http.StatusRequestEntityTooLarge, "upload exceeds 2 GiB")
		return
	}

	mode, err := strconv.ParseUint(r.Header.Get(api.UploadModeHeader), 8, 32)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file mode")
		return
	}
	checksum, err := hex.DecodeString(r.Header.Get(api.UploadSHA256Header))
	if err != nil || len(checksum) != sha256.Size {
		writeError(w, http.StatusBadRequest, "invalid sha256 checksum")
		return
	}

	agent, ok := s.dial(w, r)
	if !ok {
		return
	}
	defer agent.Close()

	written, err := agent.Upload(path, http.MaxBytesReader(w, r.Body, maxUploadSize), uint64(r.ContentLength), uint32(mode), checksum)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, api.UploadResponse{BytesWritten: written})
}
