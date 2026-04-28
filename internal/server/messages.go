package server

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// MaxMessageBodyBytes caps the payload of a single chat message. QA r5 G-2
// landed a 1.5MB body via the API and watched every dashboard page-load
// re-fetch it. 64 KiB is well above any legitimate paste-in and far below
// the threshold where the GET /api/messages response becomes painful.
const MaxMessageBodyBytes = 64 * 1024

// validThread accepts the special "global" channel and any well-formed item
// id (PREFIX-NUMBER). Anything else (path traversal, control chars, SQL-shaped
// strings) is rejected at the boundary so downstream renderers don't have to
// worry about exotic thread names.
var validThread = regexp.MustCompile(`^(global|[A-Z][A-Z0-9]*-\d+)$`)

func validChatKind(k string) bool {
	for _, allowed := range chat.AllKinds() {
		if k == allowed {
			return true
		}
	}
	return false
}

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
	         COALESCE(mentions, '[]'), priority, repo_id FROM messages WHERE 1=1`
	args := []any{}
	if s.cfg.RepoID != "" {
		sqlq += " AND repo_id = ?"
		args = append(args, s.cfg.RepoID)
	}
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
		RepoID   string          `json:"repo_id"`
	}
	out := []msgRow{}
	for rows.Next() {
		var m msgRow
		var ment string
		if err := rows.Scan(&m.ID, &m.TS, &m.AgentID, &m.Thread, &m.Kind, &m.Body, &ment, &m.Priority, &m.RepoID); err != nil {
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
		Kind     string   `json:"kind"`
		Mentions []string `json:"mentions"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxMessageBodyBytes+1024) // +1KB for envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Thread == "" {
		req.Thread = "global"
	}
	if !validThread.MatchString(req.Thread) {
		writeErr(w, http.StatusBadRequest, "thread must be 'global' or a PREFIX-NUMBER item id")
		return
	}
	if strings.TrimSpace(req.Body) == "" {
		writeErr(w, http.StatusBadRequest, "body required")
		return
	}
	if len(req.Body) > MaxMessageBodyBytes {
		writeErr(w, http.StatusRequestEntityTooLarge,
			"body exceeds limit (got "+strconv.Itoa(len(req.Body))+" bytes, max "+strconv.Itoa(MaxMessageBodyBytes)+")")
		return
	}
	if req.Kind == "" {
		req.Kind = chat.KindSay
	}
	if !validChatKind(req.Kind) {
		writeErr(w, http.StatusBadRequest, "unknown kind: "+req.Kind)
		return
	}
	mentions := req.Mentions
	if mentions == nil {
		mentions = chat.ParseMentions(req.Body)
	}
	// Workspace mode: tag the message with the right repo so per-repo
	// activity feeds reflect the post. For item threads we resolve via
	// the items table; for the global thread we honor an explicit
	// ?repo_id= and otherwise fall back to "" (which the chat layer
	// turns into the daemon's default — empty in workspace mode).
	repoOverride := r.URL.Query().Get("repo_id")
	if s.cfg.RepoID == "" && req.Thread != "global" {
		resolved, _, _, rerr := s.resolveItemRepo(r.Context(), req.Thread, repoOverride)
		if rerr == nil {
			repoOverride = resolved
		}
	}
	if err := s.chat.Post(r.Context(), chat.PostRequest{
		AgentID:  agent,
		Thread:   req.Thread,
		Kind:     req.Kind,
		Body:     req.Body,
		Mentions: mentions,
		RepoID:   repoOverride,
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
