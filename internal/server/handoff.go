package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
)

type handoffReq struct {
	To      string `json:"to"`
	Summary string `json:"summary"`
}

func (s *Server) handleItemHandoff(w http.ResponseWriter, r *http.Request) {
	agent := s.actingAgent(r)
	if agent == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}
	id := r.PathValue("id")
	var req handoffReq
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	to := strings.TrimSpace(strings.TrimPrefix(req.To, "@"))
	if to == "" {
		writeErr(w, http.StatusBadRequest, "to required")
		return
	}

	hb := chat.HandoffBody{
		InFlight: []string{id},
		Note:     req.Summary,
	}
	if hb.Empty() {
		hb.Note = "handoff " + id + " to " + to
	}
	// Post the handoff message first; it is the user-visible signal.
	// A failure to reassign (e.g. claim already released) does not
	// invalidate the post.
	if perr := s.chat.PostHandoff(r.Context(), agent, hb); perr != nil {
		writeErr(w, http.StatusInternalServerError, perr.Error())
		return
	}
	store := claims.New(s.db, s.cfg.RepoID, nil)
	rerr := store.Reassign(r.Context(), id, agent, to)
	switch {
	case rerr == nil, errors.Is(rerr, claims.ErrNotClaimed), errors.Is(rerr, claims.ErrNotYours):
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeErr(w, http.StatusInternalServerError, rerr.Error())
	}
}
