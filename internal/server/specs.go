package server

import (
	"net/http"
	"path/filepath"

	"github.com/zsiec/squad/internal/specs"
)

type specListRow struct {
	Name   string `json:"name"`
	Title  string `json:"title"`
	Path   string `json:"path"`
	RepoID string `json:"repo_id"`
}

type specWithRepo struct {
	spec   specs.Spec
	repoID string
}

func (s *Server) walkSpecsAll() ([]specWithRepo, error) {
	if s.cfg.RepoID != "" {
		all, err := specs.Walk(s.cfg.SquadDir)
		if err != nil {
			return nil, err
		}
		out := make([]specWithRepo, 0, len(all))
		for _, sp := range all {
			out = append(out, specWithRepo{spec: sp, repoID: s.cfg.RepoID})
		}
		return out, nil
	}
	rows, err := s.db.Query(`SELECT id, COALESCE(root_path, '') FROM repos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []specWithRepo
	for rows.Next() {
		var id, root string
		if err := rows.Scan(&id, &root); err != nil || root == "" {
			continue
		}
		all, err := specs.Walk(filepath.Join(root, ".squad"))
		if err != nil {
			// Skip repos whose .squad/ is unreadable rather than failing the
			// whole aggregation — operators see partial data instead of an
			// opaque 500 when one repo is missing. Mirrors items.walkAll.
			continue
		}
		for _, sp := range all {
			out = append(out, specWithRepo{spec: sp, repoID: id})
		}
	}
	return out, rows.Err()
}

func (s *Server) handleSpecsList(w http.ResponseWriter, r *http.Request) {
	all, err := s.walkSpecsAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]specListRow, 0, len(all))
	for _, sr := range all {
		out = append(out, specListRow{
			Name: sr.spec.Name, Title: sr.spec.Title, Path: sr.spec.Path,
			RepoID: sr.repoID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSpecDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	wantRepo := r.URL.Query().Get("repo_id")
	all, err := s.walkSpecsAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, sr := range all {
		sp := sr.spec
		if sp.Name != name {
			continue
		}
		if wantRepo != "" && sr.repoID != wantRepo {
			continue
		}
		if sp.Acceptance == nil {
			sp.Acceptance = []string{}
		}
		if sp.NonGoals == nil {
			sp.NonGoals = []string{}
		}
		if sp.Integration == nil {
			sp.Integration = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"name":          sp.Name,
			"title":         sp.Title,
			"motivation":    sp.Motivation,
			"acceptance":    sp.Acceptance,
			"non_goals":     sp.NonGoals,
			"integration":   sp.Integration,
			"body_markdown": sp.Body,
			"path":          sp.Path,
			"repo_id":       sr.repoID,
		})
		return
	}
	writeErr(w, http.StatusNotFound, "spec not found: "+name)
}
