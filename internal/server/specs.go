package server

import (
	"net/http"

	"github.com/zsiec/squad/internal/specs"
)

type specListRow struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

func (s *Server) handleSpecsList(w http.ResponseWriter, r *http.Request) {
	all, err := specs.Walk(s.cfg.SquadDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]specListRow, 0, len(all))
	for _, sp := range all {
		out = append(out, specListRow{Name: sp.Name, Title: sp.Title, Path: sp.Path})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSpecDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	all, err := specs.Walk(s.cfg.SquadDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, sp := range all {
		if sp.Name == name {
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
			})
			return
		}
	}
	writeErr(w, http.StatusNotFound, "spec not found: "+name)
}
