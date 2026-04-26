package server

import (
	"net/http"

	"github.com/zsiec/squad/internal/claims"
)

func (s *Server) handleItemTouch(w http.ResponseWriter, r *http.Request) {
	agent := s.actingAgent(r)
	if agent == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}
	// claims.TouchClaim updates every active claim row owned by this agent;
	// the {id} in the URL is unused at the storage layer but kept so the TUI
	// can address an item-scoped heartbeat consistently with the other verbs.
	var n int
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM claims WHERE repo_id = ? AND agent_id = ?`,
		s.cfg.RepoID, agent).Scan(&n); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == 0 {
		writeErr(w, http.StatusNotFound, "no active claim for "+agent)
		return
	}
	store := claims.New(s.db, s.cfg.RepoID, nil)
	if err := store.TouchClaim(r.Context(), agent); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
