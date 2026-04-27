package server

import (
	"database/sql"
	"net/http"

	"github.com/zsiec/squad/internal/api"
	"github.com/zsiec/squad/internal/items"
)

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	var (
		rows *sql.Rows
		err  error
	)
	if s.cfg.RepoID == "" {
		rows, err = s.db.QueryContext(r.Context(),
			`SELECT item_id, title, COALESCE(captured_by,''), COALESCE(captured_at,0),
			        COALESCE(parent_spec,''), path, repo_id
			 FROM items WHERE status='captured'
			 ORDER BY captured_at ASC`)
	} else {
		rows, err = s.db.QueryContext(r.Context(),
			`SELECT item_id, title, COALESCE(captured_by,''), COALESCE(captured_at,0),
			        COALESCE(parent_spec,''), path, repo_id
			 FROM items WHERE repo_id=? AND status='captured'
			 ORDER BY captured_at ASC`,
			s.cfg.RepoID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := make([]api.InboxEntry, 0)
	for rows.Next() {
		var e api.InboxEntry
		if err := rows.Scan(&e.ID, &e.Title, &e.CapturedBy, &e.CapturedAt, &e.ParentSpec, &e.Path, &e.RepoID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if it, perr := items.Parse(e.Path); perr == nil {
			e.DoRPass = len(items.DoRCheck(it)) == 0
			e.AutoRefinedAt = it.AutoRefinedAt
			e.AutoRefinedBy = it.AutoRefinedBy
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}
