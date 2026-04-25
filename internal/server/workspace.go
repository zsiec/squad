package server

import (
	"net/http"

	"github.com/zsiec/squad/internal/workspace"
)

type repoWireRow struct {
	RepoID string `json:"repo_id"`
	Path   string `json:"path"`
	Remote string `json:"remote"`
}

func (s *Server) handleRepos(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT id, COALESCE(root_path, ''), COALESCE(remote_url, '') FROM repos ORDER BY id`)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []repoWireRow{}
	for rows.Next() {
		var rr repoWireRow
		if err := rows.Scan(&rr.RepoID, &rr.Path, &rr.Remote); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, rr)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleWorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	repos := r.URL.Query()["repo"]
	f := workspace.Filter{Mode: workspace.ScopeAll, CurrentRepoID: s.cfg.RepoID}
	if len(repos) > 0 {
		f = workspace.Filter{Mode: workspace.ScopeExplicit, ExplicitIDs: repos, CurrentRepoID: s.cfg.RepoID}
	}
	rows, err := workspace.New(s.db).Status(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"repos":   rows,
		"summary": map[string]any{"count": len(rows)},
	})
}
