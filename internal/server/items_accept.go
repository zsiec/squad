package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/zsiec/squad/internal/items"
)

func (s *Server) handleItemsAccept(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	var body struct {
		AcceptedBy string `json:"accepted_by,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	by := body.AcceptedBy
	if by == "" {
		by = "web"
	}
	err := items.Promote(r.Context(), s.db, s.cfg.RepoID, id, by)
	if err == nil {
		s.publishInboxChanged(id, "accepted")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var dorErr *items.DoRError
	if errors.As(err, &dorErr) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"violations": dorErr.Violations,
		})
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "item not found") {
		writeErr(w, http.StatusNotFound, msg)
		return
	}
	if strings.Contains(msg, "only captured items can be promoted") {
		writeErr(w, http.StatusConflict, msg)
		return
	}
	writeErr(w, http.StatusInternalServerError, msg)
}
