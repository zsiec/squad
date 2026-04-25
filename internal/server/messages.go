package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

func (s *Server) handleMessagesList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	thread := q.Get("thread")
	limit := 100
	if lv := q.Get("limit"); lv != "" {
		if n, err := strconv.Atoi(lv); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	var sinceTS, beforeID int64
	if sv := q.Get("since"); sv != "" {
		if n, err := strconv.ParseInt(sv, 10, 64); err == nil {
			sinceTS = n
		}
	}
	if bv := q.Get("before"); bv != "" {
		if n, err := strconv.ParseInt(bv, 10, 64); err == nil {
			beforeID = n
		}
	}
	sqlq := `SELECT id, ts, agent_id, thread, kind, COALESCE(body, ''),
	         COALESCE(mentions, '[]'), priority FROM messages WHERE repo_id = ?`
	args := []any{s.cfg.RepoID}
	if thread != "" {
		sqlq += " AND thread = ?"
		args = append(args, thread)
	}
	if sinceTS > 0 {
		sqlq += " AND ts > ?"
		args = append(args, sinceTS)
	}
	if beforeID > 0 {
		sqlq += " AND id < ?"
		args = append(args, beforeID)
	}
	sqlq += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.QueryContext(r.Context(), sqlq, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	type msgRow struct {
		ID       int64           `json:"id"`
		TS       int64           `json:"ts"`
		AgentID  string          `json:"agent_id"`
		Thread   string          `json:"thread"`
		Kind     string          `json:"kind"`
		Body     string          `json:"body"`
		Mentions json.RawMessage `json:"mentions"`
		Priority string          `json:"priority"`
	}
	out := []msgRow{}
	for rows.Next() {
		var m msgRow
		var ment string
		if err := rows.Scan(&m.ID, &m.TS, &m.AgentID, &m.Thread, &m.Kind, &m.Body, &ment, &m.Priority); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		m.Mentions = json.RawMessage(ment)
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMessagesPost(w http.ResponseWriter, r *http.Request) {
	agent := s.actingAgent(r)
	if agent == "" {
		writeErr(w, http.StatusBadRequest, "X-Squad-Agent header required")
		return
	}
	if !s.allowMessage(agent) {
		writeErr(w, http.StatusTooManyRequests, "rate limited")
		return
	}
	var req struct {
		Thread   string   `json:"thread"`
		Body     string   `json:"body"`
		Mentions []string `json:"mentions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Thread == "" {
		req.Thread = "global"
	}
	if strings.TrimSpace(req.Body) == "" {
		writeErr(w, http.StatusBadRequest, "body required")
		return
	}
	mentions := req.Mentions
	if mentions == nil {
		mentions = chat.ParseMentions(req.Body)
	}
	if err := s.chat.Post(r.Context(), chat.PostRequest{
		AgentID:  agent,
		Thread:   req.Thread,
		Kind:     chat.KindSay,
		Body:     req.Body,
		Mentions: mentions,
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// allowMessage permits up to 10 messages per agent per 10s rolling window.
func (s *Server) allowMessage(agent string) bool {
	s.rlMu.Lock()
	defer s.rlMu.Unlock()
	if s.rl == nil {
		s.rl = map[string][]time.Time{}
	}
	now := time.Now()
	cut := now.Add(-10 * time.Second)
	hist := s.rl[agent][:0:0]
	for _, t := range s.rl[agent] {
		if t.After(cut) {
			hist = append(hist, t)
		}
	}
	if len(hist) >= 10 {
		s.rl[agent] = hist
		return false
	}
	s.rl[agent] = append(hist, now)
	return true
}
