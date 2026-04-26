package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/items"
)

type doneReq struct {
	Summary       string `json:"summary"`
	EvidenceForce bool   `json:"evidence_force"`
}

func (s *Server) handleItemDone(w http.ResponseWriter, r *http.Request) {
	agent := s.actingAgent(r)
	if agent == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}
	id := r.PathValue("id")
	var req doneReq
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	doneDir := filepath.Join(s.cfg.SquadDir, "done")

	itemPath := findItemPathFor(s.cfg.SquadDir, id)
	if itemPath == "" {
		writeErr(w, http.StatusNotFound, "no item file for "+id)
		return
	}
	parsed, err := items.Parse(itemPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	required := requiredAttestKinds(parsed.EvidenceRequired)
	if len(required) > 0 {
		L := attest.New(s.db, s.cfg.RepoID, nil)
		missing, mErr := L.MissingKinds(r.Context(), id, required)
		if mErr != nil {
			writeErr(w, http.StatusInternalServerError, mErr.Error())
			return
		}
		if len(missing) > 0 && !req.EvidenceForce {
			parts := make([]string, len(missing))
			for i, k := range missing {
				parts[i] = string(k)
			}
			writeErr(w, http.StatusPreconditionFailed,
				"evidence_required not satisfied for "+id+": missing "+strings.Join(parts, ", "))
			return
		}
	}

	store := claims.New(s.db, s.cfg.RepoID, nil)
	derr := store.Done(r.Context(), id, agent, claims.DoneOpts{
		Summary:  req.Summary,
		ItemPath: itemPath,
		DoneDir:  doneDir,
	})
	switch {
	case derr == nil:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case errors.Is(derr, claims.ErrNotClaimed):
		writeErr(w, http.StatusNotFound, derr.Error())
	case errors.Is(derr, claims.ErrNotYours):
		writeErr(w, http.StatusForbidden, derr.Error())
	default:
		writeErr(w, http.StatusInternalServerError, derr.Error())
	}
}

func findItemPathFor(squadDir, itemID string) string {
	w, err := items.Walk(squadDir)
	if err != nil {
		return ""
	}
	for _, it := range w.Active {
		if it.ID == itemID {
			return it.Path
		}
	}
	return ""
}

func requiredAttestKinds(raw []string) []attest.Kind {
	out := make([]attest.Kind, 0, len(raw))
	for _, r := range raw {
		k := attest.Kind(strings.TrimSpace(r))
		if k.Valid() {
			out = append(out, k)
		}
	}
	return out
}
