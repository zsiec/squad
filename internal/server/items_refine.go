package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/zsiec/squad/internal/items"
)

var errItemNotFound = errors.New("item not found")

func (s *Server) handleItemsRefine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}
	var body struct {
		Comments string `json:"comments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}

	err := items.Refine(r.Context(), s.db, s.cfg.RepoID, id, body.Comments)
	switch {
	case err == nil:
		s.publishInboxChanged(id, "refine")
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, items.ErrCommentsRequired):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, items.ErrWrongStatusForRefine):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, items.ErrItemNotFound):
		writeErr(w, http.StatusNotFound, "item not found")
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}
