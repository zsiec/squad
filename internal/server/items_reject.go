package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/items"
)

func (s *Server) handleItemsReject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	var body struct {
		Reason string `json:"reason"`
		By     string `json:"by,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		writeErr(w, http.StatusBadRequest, "reason required")
		return
	}
	by := body.By
	if by == "" {
		by = "web"
	}
	squadDir := s.cfg.SquadDir
	if squadDir == "" || squadDir == ".squad" {
		squadDir = filepath.Join(s.cfg.LearningsRoot, ".squad")
	}
	err := items.Reject(r.Context(), s.db, s.cfg.RepoID, id, body.Reason, by, squadDir)
	if errors.Is(err, items.ErrItemClaimed) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, items.ErrReasonRequired) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
