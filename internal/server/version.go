package server

import (
	"net"
	"net/http"
	"os"
	"time"
)

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackRemote(r.RemoteAddr) {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     s.cfg.Version,
		"binary_path": s.cfg.BinaryPath,
		"started_at":  s.startedAt.Format(time.RFC3339),
		"pid":         os.Getpid(),
	})
}

func isLoopbackRemote(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
