package server

import (
	"net/http"
	"path/filepath"

	"github.com/zsiec/squad/internal/epics"
)

type epicListRow struct {
	Name        string `json:"name"`
	Spec        string `json:"spec"`
	Status      string `json:"status"`
	Parallelism string `json:"parallelism"`
	Path        string `json:"path"`
	RepoID      string `json:"repo_id"`
}

type epicWithRepo struct {
	epic   epics.Epic
	repoID string
}

// walkEpicsAll mirrors walkAll for items: single-repo mode walks
// s.cfg.SquadDir directly; workspace mode (cfg.RepoID == "") enumerates
// repos via the global DB and walks each <root>/.squad.
func (s *Server) walkEpicsAll() ([]epicWithRepo, error) {
	if s.cfg.RepoID != "" {
		all, _, err := epics.Walk(s.cfg.SquadDir)
		if err != nil {
			return nil, err
		}
		out := make([]epicWithRepo, 0, len(all))
		for _, e := range all {
			out = append(out, epicWithRepo{epic: e, repoID: s.cfg.RepoID})
		}
		return out, nil
	}
	rows, err := s.db.Query(`SELECT id, COALESCE(root_path, '') FROM repos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []epicWithRepo
	for rows.Next() {
		var id, root string
		if err := rows.Scan(&id, &root); err != nil || root == "" {
			continue
		}
		all, _, err := epics.Walk(filepath.Join(root, ".squad"))
		if err != nil {
			// Skip repos whose .squad/ is unreadable rather than failing the
			// whole aggregation — operators see partial data instead of an
			// opaque 500 when one repo is missing. Mirrors items.walkAll.
			continue
		}
		for _, e := range all {
			out = append(out, epicWithRepo{epic: e, repoID: id})
		}
	}
	return out, rows.Err()
}

func (s *Server) handleEpicsList(w http.ResponseWriter, r *http.Request) {
	all, err := s.walkEpicsAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	specFilter := r.URL.Query().Get("spec")
	out := make([]epicListRow, 0, len(all))
	for _, er := range all {
		e := er.epic
		if specFilter != "" && e.Spec != specFilter {
			continue
		}
		out = append(out, epicListRow{
			Name: e.Name, Spec: e.Spec, Status: e.Status,
			Parallelism: e.Parallelism, Path: e.Path,
			RepoID: er.repoID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleEpicDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wantRepo := r.URL.Query().Get("repo_id")
	all, err := s.walkEpicsAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, er := range all {
		if er.epic.Name != name {
			continue
		}
		if wantRepo != "" && er.repoID != wantRepo {
			continue
		}
		e := er.epic
		writeJSON(w, http.StatusOK, map[string]any{
			"name":          e.Name,
			"spec":          e.Spec,
			"status":        e.Status,
			"parallelism":   e.Parallelism,
			"body_markdown": e.Body,
			"path":          e.Path,
			"repo_id":       er.repoID,
		})
		return
	}
	writeErr(w, http.StatusNotFound, "epic not found: "+name)
}
