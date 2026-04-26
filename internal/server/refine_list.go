package server

import "net/http"

type refineEntry struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	CapturedBy string `json:"captured_by,omitempty"`
	CapturedAt int64  `json:"captured_at,omitempty"`
	RefinedAt  int64  `json:"refined_at,omitempty"`
}

func (s *Server) handleRefineList(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT item_id, title, COALESCE(captured_by,''), COALESCE(captured_at,0), COALESCE(updated_at,0)
		 FROM items WHERE repo_id=? AND status='needs-refinement'
		 ORDER BY updated_at ASC`,
		s.cfg.RepoID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := make([]refineEntry, 0)
	for rows.Next() {
		var e refineEntry
		if err := rows.Scan(&e.ID, &e.Title, &e.CapturedBy, &e.CapturedAt, &e.RefinedAt); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}
