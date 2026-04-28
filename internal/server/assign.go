package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/claims"
)

type assignReq struct {
	AgentID string `json:"agent_id"`
	Intent  string `json:"intent"`
}

func (s *Server) handleItemAssign(w http.ResponseWriter, r *http.Request) {
	operator := s.actingAgent(r)
	if operator == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}
	id := r.PathValue("id")
	var req assignReq
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	target := strings.TrimSpace(req.AgentID)
	if target == "" {
		writeErr(w, http.StatusBadRequest, "agent_id required")
		return
	}

	repoID, squadDir, statusCode, rerr := s.resolveItemRepo(r.Context(), id, r.URL.Query().Get("repo_id"))
	if rerr != nil {
		writeResolveErr(w, statusCode, rerr)
		return
	}

	if err := assertAgentRegistered(r.Context(), s.db, repoID, target); err != nil {
		writeErr(w, http.StatusNotFound, err.Error())
		return
	}

	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "assigned by " + operator
	}

	store := claims.New(s.db, repoID, nil)
	itemsDir := filepath.Join(squadDir, "items")
	doneDir := filepath.Join(squadDir, "done")
	err := store.Claim(r.Context(), id, target, intent, nil, false,
		claims.ClaimWithPreflight(itemsDir, doneDir))
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case errors.Is(err, claims.ErrClaimTaken),
		errors.Is(err, claims.ErrConflictsWithActive):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, claims.ErrItemNotFound),
		errors.Is(err, claims.ErrItemAlreadyDone):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, claims.ErrBlockedByOpen):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func assertAgentRegistered(ctx context.Context, db *sql.DB, repoID, agentID string) error {
	var seen string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM agents WHERE id = ? AND repo_id = ?`,
		agentID, repoID).Scan(&seen)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("target agent not registered: " + agentID)
	}
	return err
}
