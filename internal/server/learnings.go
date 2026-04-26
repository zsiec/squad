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
}

func (s *Server) handleLearningsList(w http.ResponseWriter, r *http.Request) {
	all, err := learning.Walk(s.cfg.LearningsRoot)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	q := r.URL.Query()
	stateFilter := q.Get("state")
	kindFilter := q.Get("kind")
	areaFilter := q.Get("area")
	out := make([]learningRow, 0, len(all))
	for _, l := range all {
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
			Paths: paths, Related: related,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
