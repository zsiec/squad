package server

import (
	"crypto/subtle"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	restartTokenHeader      = "X-Squad-Restart-Token"
	defaultRestartExitDelay = 200 * time.Millisecond
)

func (s *Server) handleInternalRestart(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRemote(r.RemoteAddr) {
		http.NotFound(w, r)
		return
	}
	want, err := os.ReadFile(s.resolveRestartTokenPath())
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	got := strings.TrimSpace(r.Header.Get(restartTokenHeader))
	wantTrimmed := strings.TrimSpace(string(want))
	if got == "" || wantTrimmed == "" || subtle.ConstantTimeCompare([]byte(got), []byte(wantTrimmed)) != 1 {
		writeErr(w, http.StatusUnauthorized, "invalid token")
		return
	}
	w.WriteHeader(http.StatusAccepted)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	delay := s.restartExitDelay
	if delay <= 0 {
		delay = defaultRestartExitDelay
	}
	exit := s.exitFunc
	if exit == nil {
		exit = func() { os.Exit(0) }
	}
	time.AfterFunc(delay, exit)
}

func (s *Server) resolveRestartTokenPath() string {
	if s.cfg.RestartTokenPath != "" {
		return s.cfg.RestartTokenPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".squad", "restart.token")
}
