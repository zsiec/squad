package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/items"
)

// serverTypeToPrefix mirrors cmd/squad/new.go::typeToPrefix. Duplicated for
// now; fold to internal/items.PrefixFor once a third caller appears.
var serverTypeToPrefix = map[string]string{
	"bug":       "BUG",
	"feature":   "FEAT",
	"feat":      "FEAT",
	"task":      "TASK",
	"chore":     "CHORE",
	"tech-debt": "DEBT",
	"debt":      "DEBT",
	"bet":       "BET",
}

func (s *Server) handleItemsCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type       string `json:"type"`
		Title      string `json:"title"`
		Priority   string `json:"priority,omitempty"`
		Area       string `json:"area,omitempty"`
		Estimate   string `json:"estimate,omitempty"`
		Risk       string `json:"risk,omitempty"`
		Ready      bool   `json:"ready,omitempty"`
		CapturedBy string `json:"captured_by,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "decode body: "+err.Error())
		return
	}
	if strings.TrimSpace(body.Title) == "" || strings.TrimSpace(body.Type) == "" {
		writeErr(w, http.StatusBadRequest, "type and title are required")
		return
	}
	typ := strings.ToLower(body.Type)
	prefix, ok := serverTypeToPrefix[typ]
	if !ok {
		prefix = strings.ToUpper(typ)
	}
	repoRoot := s.cfg.LearningsRoot
	cfg, err := config.Load(repoRoot)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load config: "+err.Error())
		return
	}
	if !containsString(cfg.IDPrefixes, prefix) {
		writeErr(w, http.StatusBadRequest,
			fmt.Sprintf("type %q maps to prefix %q which is not in id_prefixes %v",
				typ, prefix, cfg.IDPrefixes))
		return
	}
	capturedBy := body.CapturedBy
	if capturedBy == "" {
		capturedBy = "web"
	}
	squadDir := s.cfg.SquadDir
	if squadDir == "" || squadDir == ".squad" {
		squadDir = filepath.Join(repoRoot, ".squad")
	}
	path, err := items.NewWithOptions(squadDir, prefix, body.Title, items.Options{
		Priority:   nonEmpty(body.Priority, cfg.Defaults.Priority),
		Estimate:   nonEmpty(body.Estimate, cfg.Defaults.Estimate),
		Risk:       nonEmpty(body.Risk, cfg.Defaults.Risk),
		Area:       nonEmpty(body.Area, cfg.Defaults.Area),
		Ready:      body.Ready,
		CapturedBy: capturedBy,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "new item: "+err.Error())
		return
	}
	parsed, err := items.Parse(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "parse: "+err.Error())
		return
	}
	if err := items.Persist(r.Context(), s.db, s.cfg.RepoID, parsed, false); err != nil {
		writeErr(w, http.StatusInternalServerError, "persist: "+err.Error())
		return
	}
	s.publishInboxChanged(parsed.ID, "captured")
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     parsed.ID,
		"status": parsed.Status,
		"path":   path,
	})
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func nonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
