package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/store"
)

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
	if strings.TrimSpace(body.Comments) == "" {
		writeErr(w, http.StatusUnprocessableEntity, "comments required")
		return
	}

	err := store.WithTxRetry(r.Context(), s.db, func(tx *sql.Tx) error {
		var path, status string
		if err := tx.QueryRowContext(r.Context(),
			`SELECT path, status FROM items WHERE repo_id=? AND item_id=?`,
			s.cfg.RepoID, id,
		).Scan(&path, &status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errItemNotFound
			}
			return err
		}
		if status != "captured" && status != "needs-refinement" {
			return fmt.Errorf("refine %s: status is %q (only captured or needs-refinement items can be refined)", id, status)
		}
		if err := items.RewriteWithFeedback(path, body.Comments, "needs-refinement", time.Now()); err != nil {
			return err
		}
		it, err := items.Parse(path)
		if err != nil {
			return err
		}
		return items.PersistOne(r.Context(), tx, s.cfg.RepoID, it, false, time.Now().Unix())
	})
	if errors.Is(err, errItemNotFound) {
		writeErr(w, http.StatusNotFound, "item not found")
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "only captured or needs-refinement") {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.publishInboxChanged(id, "refine")
	w.WriteHeader(http.StatusNoContent)
}

var errItemNotFound = errors.New("item not found")
