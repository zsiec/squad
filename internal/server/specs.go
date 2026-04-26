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
