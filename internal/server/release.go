package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/zsiec/squad/internal/claims"
)

type releaseReq struct {
	Outcome string `json:"outcome"`
}

func (s *Server) handleItemRelease(w http.ResponseWriter, r *http.Request) {
	agent := s.actingAgent(r)
	if agent == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}
	id := r.PathValue("id")
	var req releaseReq
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Outcome == "" {
		req.Outcome = "released"
	}

	repoID, _, statusCode, rerr := s.resolveItemRepo(r.Context(), id, r.URL.Query().Get("repo_id"))
	if rerr != nil {
		writeResolveErr(w, statusCode, rerr)
		return
	}
	store := claims.New(s.db, repoID, nil)
	err := store.Release(r.Context(), id, agent, req.Outcome)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case errors.Is(err, claims.ErrNotClaimed):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, claims.ErrNotYours):
		writeErr(w, http.StatusForbidden, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}
