package server

import (
	"errors"
	"net/http"

	"github.com/zsiec/squad/internal/items"
)

func (s *Server) handleItemsRecapture(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	agent := s.actingAgent(r)
	if agent == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}

	err := items.Recapture(r.Context(), s.db, s.cfg.RepoID, id, agent)
	switch {
	case err == nil:
		s.publishInboxChanged(id, "recapture")
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, items.ErrItemNotFound):
		writeErr(w, http.StatusNotFound, "item not found")
	case errors.Is(err, items.ErrClaimNotHeld):
		writeErr(w, http.StatusForbidden, "claim not held by "+agent)
	case errors.Is(err, items.ErrWrongStatusForRecapture):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}
