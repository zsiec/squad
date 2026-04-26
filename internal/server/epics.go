package server

import (
	"net/http"

	"github.com/zsiec/squad/internal/epics"
)

type epicListRow struct {
	Name        string `json:"name"`
	Spec        string `json:"spec"`
	Status      string `json:"status"`
	Parallelism string `json:"parallelism"`
	Path        string `json:"path"`
}

func (s *Server) handleEpicsList(w http.ResponseWriter, r *http.Request) {
	all, _, err := epics.Walk(s.cfg.SquadDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	specFilter := r.URL.Query().Get("spec")
	out := make([]epicListRow, 0, len(all))
	for _, e := range all {
		if specFilter != "" && e.Spec != specFilter {
			continue
		}
		out = append(out, epicListRow{
			Name: e.Name, Spec: e.Spec, Status: e.Status,
			Parallelism: e.Parallelism, Path: e.Path,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
