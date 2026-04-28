package server

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// handleItemActivity returns the chat-message stream for an item's thread.
// The SPA's drawer fetches /api/items/{id}/activity?limit=N to render the
// item-detail activity panel; without this endpoint the drawer shows
// "Error: /api/items/.../activity?limit=80: 404". Messages come back
// newest-first; the SPA reverses for display.
//
// Query params:
//   - limit  — max rows (default 100, max 500)
//   - before — ts cutoff for pagination (return rows with ts < before)
//
// v1 returns chat messages only. Structured events (claim/release/done from
// claim_history; attestations from the attest ledger) merge in later under
// the same endpoint shape — view modules already pattern-match on kind.
func (s *Server) handleItemActivity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	q := r.URL.Query()
	repoID, _, statusCode, rerr := s.resolveItemRepo(r.Context(), id, q.Get("repo_id"))
	if rerr != nil {
		writeResolveErr(w, statusCode, rerr)
		return
	}
	limit := 100
	if lv := q.Get("limit"); lv != "" {
		if n, err := strconv.Atoi(lv); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	var beforeTS int64
	if bv := q.Get("before"); bv != "" {
		if n, err := strconv.ParseInt(bv, 10, 64); err == nil {
			beforeTS = n
		}
	}

	sqlq := `SELECT id, ts, agent_id, kind, COALESCE(body, ''), COALESCE(mentions, '[]'), repo_id
		FROM messages WHERE thread = ? AND repo_id = ?`
	args := []any{id, repoID}
	if beforeTS > 0 {
		sqlq += " AND ts < ?"
		args = append(args, beforeTS)
	}
	sqlq += " ORDER BY ts DESC, id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(r.Context(), sqlq, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type evt struct {
		ID       int64           `json:"id"`
		TS       int64           `json:"ts"`
		AgentID  string          `json:"agent_id"`
		Kind     string          `json:"kind"`
		Body     string          `json:"body"`
		Mentions json.RawMessage `json:"mentions"`
		RepoID   string          `json:"repo_id"`
	}
	out := []evt{}
	for rows.Next() {
		var e evt
		var ment string
		if err := rows.Scan(&e.ID, &e.TS, &e.AgentID, &e.Kind, &e.Body, &ment, &e.RepoID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		e.Mentions = json.RawMessage(ment)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
