package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"

	"github.com/zsiec/squad/internal/claims"
)

type claimReq struct {
	Intent  string   `json:"intent"`
	Long    bool     `json:"long"`
	Touches []string `json:"touches"`
}

func (s *Server) handleItemClaim(w http.ResponseWriter, r *http.Request) {
	agent := s.actingAgent(r)
	if agent == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}
	id := r.PathValue("id")
	var req claimReq
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	store := claims.New(s.db, s.cfg.RepoID, nil)
	itemsDir := filepath.Join(s.cfg.SquadDir, "items")
	doneDir := filepath.Join(s.cfg.SquadDir, "done")
	err := store.Claim(r.Context(), id, agent, req.Intent, req.Touches, req.Long,
		claims.ClaimWithPreflight(itemsDir, doneDir))
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case errors.Is(err, claims.ErrClaimTaken),
		errors.Is(err, claims.ErrConflictsWithActive):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, claims.ErrItemNotFound),
		errors.Is(err, claims.ErrItemAlreadyDone):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, claims.ErrBlockedByOpen):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}
