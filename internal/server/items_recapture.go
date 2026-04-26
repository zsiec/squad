package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/store"
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

	var notHeld bool
	err := store.WithTxRetry(r.Context(), s.db, func(tx *sql.Tx) error {
		notHeld = false
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
		if status != "needs-refinement" {
			return fmt.Errorf("recapture %s: status is %q (only needs-refinement items can be recaptured)", id, status)
		}

		var holder string
		err := tx.QueryRowContext(r.Context(),
			`SELECT agent_id FROM claims WHERE repo_id=? AND item_id=?`,
			s.cfg.RepoID, id,
		).Scan(&holder)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && holder != agent) {
			notHeld = true
			return errClaimNotHeld
		}
		if err != nil {
			return err
		}

		now := time.Now()
		date := now.UTC().Format("2006-01-02")
		if err := items.RewriteRecapture(path, date, "captured", now); err != nil {
			return err
		}
		it, err := items.Parse(path)
		if err != nil {
			return err
		}
		if err := items.PersistOne(r.Context(), tx, s.cfg.RepoID, it, false, now.Unix()); err != nil {
			return err
		}
		_, err = tx.ExecContext(r.Context(),
			`DELETE FROM claims WHERE repo_id=? AND item_id=?`,
			s.cfg.RepoID, id,
		)
		return err
	})
	if errors.Is(err, errItemNotFound) {
		writeErr(w, http.StatusNotFound, "item not found")
		return
	}
	if notHeld {
		writeErr(w, http.StatusForbidden, "claim not held by "+agent)
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "only needs-refinement") {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.publishInboxChanged(id, "recapture")
	w.WriteHeader(http.StatusNoContent)
}

var errClaimNotHeld = errors.New("claim not held")
