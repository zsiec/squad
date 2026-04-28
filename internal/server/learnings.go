package server

import (
	"net/http"

	"github.com/zsiec/squad/internal/learning"
)

type learningRow struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Area      string   `json:"area"`
	State     string   `json:"state"`
	Created   string   `json:"created"`
	CreatedBy string   `json:"created_by"`
	Paths     []string `json:"paths"`
	Related   []string `json:"related_items"`
	RepoID    string   `json:"repo_id"`
}

type learningWithRepo struct {
	learning learning.Learning
	repoID   string
}

func (s *Server) walkLearningsAll() ([]learningWithRepo, error) {
	if s.cfg.RepoID != "" {
		all, err := learning.Walk(s.cfg.LearningsRoot)
		if err != nil {
			return nil, err
		}
		out := make([]learningWithRepo, 0, len(all))
		for _, l := range all {
			out = append(out, learningWithRepo{learning: l, repoID: s.cfg.RepoID})
		}
		return out, nil
	}
	rows, err := s.db.Query(`SELECT id, COALESCE(root_path, '') FROM repos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []learningWithRepo
	for rows.Next() {
		var id, root string
		if err := rows.Scan(&id, &root); err != nil || root == "" {
			continue
		}
		all, err := learning.Walk(root)
		if err != nil {
			// Skip repos whose .squad/learnings is unreadable rather than
			// failing the whole aggregation — operators see partial data
			// instead of an opaque 500 when one repo is missing. Mirrors
			// items.walkAll.
			continue
		}
		for _, l := range all {
			out = append(out, learningWithRepo{learning: l, repoID: id})
		}
	}
	return out, rows.Err()
}

func (s *Server) handleLearningsList(w http.ResponseWriter, r *http.Request) {
	all, err := s.walkLearningsAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	q := r.URL.Query()
	stateFilter := q.Get("state")
	kindFilter := q.Get("kind")
	areaFilter := q.Get("area")
	out := make([]learningRow, 0, len(all))
	for _, lr := range all {
		l := lr.learning
		if stateFilter != "" && string(l.State) != stateFilter {
			continue
		}
		if kindFilter != "" && string(l.Kind) != kindFilter {
			continue
		}
		if areaFilter != "" && l.Area != areaFilter {
			continue
		}
		paths := l.Paths
		if paths == nil {
			paths = []string{}
		}
		related := l.RelatedItems
		if related == nil {
			related = []string{}
		}
		out = append(out, learningRow{
			ID: l.ID, Kind: string(l.Kind), Slug: l.Slug, Title: l.Title,
			Area: l.Area, State: string(l.State),
			Created: l.Created, CreatedBy: l.CreatedBy,
			Paths: paths, Related: related, RepoID: lr.repoID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleLearningDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	wantRepo := r.URL.Query().Get("repo_id")
	all, err := s.walkLearningsAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, lr := range all {
		l := lr.learning
		if l.Slug != slug {
			continue
		}
		if wantRepo != "" && lr.repoID != wantRepo {
			continue
		}
		paths := l.Paths
		if paths == nil {
			paths = []string{}
		}
		related := l.RelatedItems
		if related == nil {
			related = []string{}
		}
		evidence := l.Evidence
		if evidence == nil {
			evidence = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":            l.ID,
			"kind":          string(l.Kind),
			"slug":          l.Slug,
			"title":         l.Title,
			"area":          l.Area,
			"state":         string(l.State),
			"created":       l.Created,
			"created_by":    l.CreatedBy,
			"session":       l.Session,
			"paths":         paths,
			"evidence":      evidence,
			"related_items": related,
			"body_markdown": l.Body,
			"path":          l.Path,
			"repo_id":       lr.repoID,
		})
		return
	}
	writeErr(w, http.StatusNotFound, "learning not found: "+slug)
}
