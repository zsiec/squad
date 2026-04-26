package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/zsiec/squad/internal/stats"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	window := 24 * time.Hour
	if v := r.URL.Query().Get("window"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			window = time.Duration(n) * time.Second
		} else if d, err := time.ParseDuration(v); err == nil {
			window = d
		}
	}
	snap, err := stats.Compute(r.Context(), s.db, stats.ComputeOpts{
		RepoID: s.cfg.RepoID, Window: window,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
