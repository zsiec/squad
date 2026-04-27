package server

import (
	"net/http"
	"os"
	"time"
)

const defaultRestartExitDelay = 200 * time.Millisecond

func (s *Server) handleInternalRestart(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRemote(r.RemoteAddr) {
		http.NotFound(w, r)
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
