package server

import (
	"net/http"

	"github.com/zsiec/squad/internal/items"
)

type inboxEntry struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	CapturedBy string `json:"captured_by,omitempty"`
	CapturedAt int64  `json:"captured_at,omitempty"`
	ParentSpec string `json:"parent_spec,omitempty"`
	DoRPass    bool   `json:"dor_pass"`
	Path       string `json:"path"`
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT item_id, title, COALESCE(captured_by,''), COALESCE(captured_at,0),
		        COALESCE(parent_spec,''), path
		 FROM items WHERE repo_id=? AND status='captured'
		 ORDER BY captured_at ASC`,
		s.cfg.RepoID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := make([]inboxEntry, 0)
	for rows.Next() {
		var e inboxEntry
		if err := rows.Scan(&e.ID, &e.Title, &e.CapturedBy, &e.CapturedAt, &e.ParentSpec, &e.Path); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if it, perr := items.Parse(e.Path); perr == nil {
			e.DoRPass = len(items.DoRCheck(it)) == 0
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}
