package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type searchHit struct {
	Kind    string  `json:"kind"`
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	RepoID  string  `json:"repo_id,omitempty"`
}

const searchSnippetMax = 160

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeErr(w, http.StatusBadRequest, "q required")
		return
	}
	limit := 40
	if lv := r.URL.Query().Get("limit"); lv != "" {
		if n, err := strconv.Atoi(lv); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	needle := strings.ToLower(q)

	hits := []searchHit{}

	all, err := s.walkAll()
	if err == nil {
		for _, it := range all {
			score := scoreItem(needle, it.ID, it.Title, it.Area, it.Body)
			if score == 0 {
				continue
			}
			snip := snippetAround(it.Body, needle)
			if snip == "" {
				snip = it.Title
			}
			hits = append(hits, searchHit{
				Kind: "item", ID: it.ID, Title: it.Title, Snippet: snip, Score: score,
				RepoID: it.RepoID,
			})
		}
	}

	msgQ := `SELECT id, agent_id, thread, COALESCE(body,''), repo_id FROM messages WHERE LOWER(body) LIKE ?`
	msgArgs := []any{"%" + needle + "%"}
	if s.cfg.RepoID != "" {
		msgQ += " AND repo_id = ?"
		msgArgs = append(msgArgs, s.cfg.RepoID)
	}
	msgQ += " ORDER BY id DESC LIMIT 200"
	rows, err := s.db.QueryContext(r.Context(), msgQ, msgArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var agent, thread, body, repoID string
			if err := rows.Scan(&id, &agent, &thread, &body, &repoID); err != nil {
				continue
			}
			snip := snippetAround(body, needle)
			title := agent + " in #" + thread
			hits = append(hits, searchHit{
				Kind: "message", ID: strconv.FormatInt(id, 10),
				Title: title, Snippet: snip, Score: 10, RepoID: repoID,
			})
		}
	}

	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	writeJSON(w, http.StatusOK, hits)
}

func scoreItem(needle, id, title, area, body string) float64 {
	idLow := strings.ToLower(id)
	tiLow := strings.ToLower(title)
	arLow := strings.ToLower(area)
	bdLow := strings.ToLower(body)

	score := 0.0
	switch {
	case idLow == needle:
		score += 200
	case strings.HasPrefix(idLow, needle):
		score += 120
	case strings.Contains(idLow, needle):
		score += 60
	}
	switch {
	case strings.HasPrefix(tiLow, needle):
		score += 80
	case strings.Contains(tiLow, needle):
		score += 40
	}
	if strings.Contains(arLow, needle) {
		score += 15
	}
	if strings.Contains(bdLow, needle) {
		score += 8
	}
	return score
}

func snippetAround(text, needle string) string {
	low := strings.ToLower(text)
	idx := strings.Index(low, needle)
	if idx < 0 {
		if len(text) <= searchSnippetMax {
			return strings.ReplaceAll(text, "\n", " ")
		}
		return strings.ReplaceAll(text[:searchSnippetMax], "\n", " ") + "…"
	}
	start := idx - 40
	if start < 0 {
		start = 0
	}
	end := idx + len(needle) + 80
	if end > len(text) {
		end = len(text)
	}
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "…"
	}
	if end < len(text) {
		suffix = "…"
	}
	return prefix + strings.ReplaceAll(text[start:end], "\n", " ") + suffix
}
