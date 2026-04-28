package server

import (
	"encoding/json"
	"errors"
	"net/http"
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
	repoID, squadDir, statusCode, rerr := s.resolveItemRepo(r.Context(), id, r.URL.Query().Get("repo_id"))
	if rerr != nil {
		writeResolveErr(w, statusCode, rerr)
		return
	}
	err := items.Reject(r.Context(), s.db, repoID, id, body.Reason, by, squadDir)
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
	s.publishInboxChanged(id, "rejected")
	w.WriteHeader(http.StatusNoContent)
}
