package server

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/zsiec/squad/internal/items"
)

type itemListRow struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Type             string   `json:"type"`
	Priority         string   `json:"priority"`
	Area             string   `json:"area"`
	Status           string   `json:"status"`
	Estimate         string   `json:"estimate"`
	Risk             string   `json:"risk"`
	Created          string   `json:"created"`
	Updated          string   `json:"updated"`
	ACTotal          int      `json:"ac_total"`
	ACChecked        int      `json:"ac_checked"`
	Progress         int      `json:"progress_pct"`
	Epic             string   `json:"epic"`
	DependsOn        []string `json:"depends_on"`
	Parallel         bool     `json:"parallel"`
	EvidenceRequired []string `json:"evidence_required"`
	ClaimedBy        string   `json:"claimed_by"`
	LastTouch        int64    `json:"last_touch"`
}

// walkAll returns all active + done items below s.cfg.SquadDir.
func (s *Server) walkAll() ([]items.Item, error) {
	w, err := items.Walk(s.cfg.SquadDir)
	if err != nil {
		return nil, err
	}
	out := make([]items.Item, 0, len(w.Active)+len(w.Done))
	out = append(out, w.Active...)
	out = append(out, w.Done...)
	return out, nil
}

func (s *Server) handleItemsList(w http.ResponseWriter, r *http.Request) {
	all, err := s.walkAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	type claimInfo struct {
		Agent     string
		LastTouch int64
	}
	claimByItem := map[string]claimInfo{}
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT item_id, agent_id, last_touch FROM claims WHERE repo_id = ?`, s.cfg.RepoID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for rows.Next() {
		var id, agent string
		var lt int64
		if err := rows.Scan(&id, &agent, &lt); err == nil {
			claimByItem[id] = claimInfo{Agent: agent, LastTouch: lt}
		}
	}
	rows.Close()

	statusFilter := r.URL.Query().Get("status")
	epicFilter := r.URL.Query().Get("epic")
	out := make([]itemListRow, 0, len(all))
	for _, it := range all {
		if statusFilter != "" && it.Status != statusFilter {
			continue
		}
		if epicFilter != "" && it.Epic != epicFilter {
			continue
		}
		deps := it.DependsOn
		if deps == nil {
			deps = []string{}
		}
		evReq := it.EvidenceRequired
		if evReq == nil {
			evReq = []string{}
		}
		row := itemListRow{
			ID: it.ID, Title: it.Title, Type: it.Type, Priority: it.Priority,
			Area: it.Area, Status: it.Status, Estimate: it.Estimate, Risk: it.Risk,
			Created: it.Created, Updated: it.Updated,
			ACTotal: it.ACTotal, ACChecked: it.ACChecked, Progress: it.ProgressPct(),
			Epic: it.Epic, DependsOn: deps, Parallel: it.Parallel, EvidenceRequired: evReq,
		}
		if c, ok := claimByItem[it.ID]; ok {
			row.ClaimedBy = c.Agent
			row.LastTouch = c.LastTouch
		}
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleItemDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	all, err := s.walkAll()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	var it *items.Item
	for i := range all {
		if all[i].ID == id {
			it = &all[i]
			break
		}
	}
	if it == nil {
		writeErr(w, http.StatusNotFound, "item not found: "+id)
		return
	}

	var currentClaim any
	var lastTouch int64
	row := s.db.QueryRowContext(r.Context(),
		`SELECT agent_id, COALESCE(intent, ''), claimed_at, last_touch FROM claims WHERE item_id = ? AND repo_id = ?`,
		id, s.cfg.RepoID)
	var cc struct {
		AgentID   string `json:"agent_id"`
		Intent    string `json:"intent"`
		ClaimedAt int64  `json:"claimed_at"`
	}
	switch err := row.Scan(&cc.AgentID, &cc.Intent, &cc.ClaimedAt, &lastTouch); {
	case err == nil:
		currentClaim = cc
	case errors.Is(err, sql.ErrNoRows):
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	if it.ACItems == nil {
		it.ACItems = []items.ACItem{}
	}
	if it.BlockedBy == nil {
		it.BlockedBy = []string{}
	}
	if it.RelatesTo == nil {
		it.RelatesTo = []string{}
	}
	if it.References == nil {
		it.References = []string{}
	}
	deps := it.DependsOn
	if deps == nil {
		deps = []string{}
	}
	evReq := it.EvidenceRequired
	if evReq == nil {
		evReq = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id": it.ID, "title": it.Title, "type": it.Type, "priority": it.Priority,
		"area": it.Area, "status": it.Status, "estimate": it.Estimate, "risk": it.Risk,
		"created": it.Created, "updated": it.Updated,
		"ac_total": it.ACTotal, "ac_checked": it.ACChecked, "progress_pct": it.ProgressPct(),
		"body_markdown": it.Body, "ac": it.ACItems,
		"blocked_by": it.BlockedBy, "relates_to": it.RelatesTo, "references": it.References,
		"current_claim":     currentClaim,
		"epic":              it.Epic,
		"depends_on":        deps,
		"parallel":          it.Parallel,
		"evidence_required": evReq,
		"last_touch":        lastTouch,
	})
}
