package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strconv"
)

const (
	defaultEventsLimit = 100
	maxEventsLimit     = 500
)

type agentEventRow struct {
	TS         int64  `json:"ts"`
	EventKind  string `json:"event_kind"`
	Tool       string `json:"tool"`
	Target     string `json:"target"`
	ExitCode   int    `json:"exit_code"`
	DurationMS int64  `json:"duration_ms"`
	SessionID  string `json:"session_id"`
}

func (s *Server) handleAgentEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.agentExists(r.Context(), id) {
		writeErr(w, http.StatusNotFound, "agent not found: "+id)
		return
	}
	since, hasSince, sErr := parseSince(r)
	if sErr != nil {
		writeErr(w, http.StatusBadRequest, sErr.Error())
		return
	}
	limit, lErr := parseLimit(r)
	if lErr != nil {
		writeErr(w, http.StatusBadRequest, lErr.Error())
		return
	}

	order := "DESC"
	args := []any{s.cfg.RepoID, id}
	where := "repo_id = ? AND agent_id = ?"
	if hasSince {
		where += " AND ts >= ?"
		args = append(args, since)
		order = "ASC"
	}
	args = append(args, limit)
	q := `
		SELECT ts, event_kind, tool, target, exit_code, duration_ms, session_id
		FROM agent_events
		WHERE ` + where + `
		ORDER BY ts ` + order + `, id ` + order + `
		LIMIT ?
	`
	rows, err := s.db.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	out := []agentEventRow{}
	for rows.Next() {
		var ev agentEventRow
		if err := rows.Scan(&ev.TS, &ev.EventKind, &ev.Tool, &ev.Target, &ev.ExitCode, &ev.DurationMS, &ev.SessionID); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, ev)
	}
	body := map[string]any{"events": out, "next_cursor": nextCursorTS(out, hasSince)}
	writeJSON(w, http.StatusOK, body)
}

type timelineRow struct {
	TS         int64  `json:"ts"`
	Kind       string `json:"kind"`
	AgentID    string `json:"agent_id"`
	Source     string `json:"source"`
	ItemID     string `json:"item_id,omitempty"`
	Thread     string `json:"thread,omitempty"`
	Body       string `json:"body,omitempty"`
	Tool       string `json:"tool,omitempty"`
	Target     string `json:"target,omitempty"`
	Outcome    string `json:"outcome,omitempty"`
	SHA        string `json:"sha,omitempty"`
	Subject    string `json:"subject,omitempty"`
	AttKind    string `json:"attestation_kind,omitempty"`
	EventKind  string `json:"event_kind,omitempty"`
	Intent     string `json:"intent,omitempty"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
}

func (s *Server) handleAgentTimeline(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.agentExists(r.Context(), id) {
		writeErr(w, http.StatusNotFound, "agent not found: "+id)
		return
	}
	since, hasSince, sErr := parseSince(r)
	if sErr != nil {
		writeErr(w, http.StatusBadRequest, sErr.Error())
		return
	}
	limit, lErr := parseLimit(r)
	if lErr != nil {
		writeErr(w, http.StatusBadRequest, lErr.Error())
		return
	}

	rows, err := s.queryTimeline(r.Context(), id, since, hasSince, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timeline": rows})
}

// queryTimeline assembles the per-agent rollup. SQLite lacks a clean way to
// UNION heterogeneous columns; running one query per source and merging in
// Go is simpler, easier to read, and (since each source has a (repo, agent,
// ts) index) cheap. The discriminator suffix on the cursor keeps cross-table
// ts ties stable: rows with equal ts always appear in source-name order.
func (s *Server) queryTimeline(ctx context.Context, agentID string, since int64, hasSince bool, limit int) ([]timelineRow, error) {
	var all []timelineRow
	tsClause := ""
	tsArgs := []any{}
	if hasSince {
		tsClause = " AND ts >= ?"
		tsArgs = []any{since}
	}

	queries := []struct {
		name string
		sql  string
		scan func(*sql.Rows, int64) (timelineRow, error)
	}{
		{
			name: "chat",
			sql: `SELECT ts, thread, kind, COALESCE(body,'')
			      FROM messages
			      WHERE repo_id = ? AND agent_id = ?` + tsClause,
			scan: func(rows *sql.Rows, ts int64) (timelineRow, error) {
				row := timelineRow{Source: "chat", AgentID: agentID, Kind: "chat"}
				var thread, kind, body string
				if err := rows.Scan(&row.TS, &thread, &kind, &body); err != nil {
					return row, err
				}
				row.Thread = thread
				row.Outcome = kind
				row.Body = body
				return row, nil
			},
		},
		{
			name: "claim",
			sql: `SELECT claimed_at, item_id, COALESCE(intent,'')
			      FROM claims
			      WHERE repo_id = ? AND agent_id = ?` + replaceTSClause(tsClause, "claimed_at"),
			scan: func(rows *sql.Rows, ts int64) (timelineRow, error) {
				row := timelineRow{Source: "claim", AgentID: agentID, Kind: "claim"}
				if err := rows.Scan(&row.TS, &row.ItemID, &row.Intent); err != nil {
					return row, err
				}
				return row, nil
			},
		},
		{
			name: "release",
			sql: `SELECT released_at, item_id, outcome
			      FROM claim_history
			      WHERE repo_id = ? AND agent_id = ?` + replaceTSClause(tsClause, "released_at"),
			scan: func(rows *sql.Rows, ts int64) (timelineRow, error) {
				row := timelineRow{Source: "release", AgentID: agentID, Kind: "release"}
				var outcome string
				if err := rows.Scan(&row.TS, &row.ItemID, &outcome); err != nil {
					return row, err
				}
				row.Outcome = outcome
				if outcome == "done" {
					row.Kind = "done"
				}
				return row, nil
			},
		},
		{
			name: "commit",
			sql: `SELECT ts, item_id, sha, subject
			      FROM commits
			      WHERE repo_id = ? AND agent_id = ?` + tsClause,
			scan: func(rows *sql.Rows, ts int64) (timelineRow, error) {
				row := timelineRow{Source: "commit", AgentID: agentID, Kind: "commit"}
				if err := rows.Scan(&row.TS, &row.ItemID, &row.SHA, &row.Subject); err != nil {
					return row, err
				}
				return row, nil
			},
		},
		{
			name: "attestation",
			sql: `SELECT created_at, item_id, kind, exit_code
			      FROM attestations
			      WHERE repo_id = ? AND agent_id = ?` + replaceTSClause(tsClause, "created_at"),
			scan: func(rows *sql.Rows, ts int64) (timelineRow, error) {
				row := timelineRow{Source: "attestation", AgentID: agentID, Kind: "attestation"}
				var attKind string
				var exit int
				if err := rows.Scan(&row.TS, &row.ItemID, &attKind, &exit); err != nil {
					return row, err
				}
				row.AttKind = attKind
				row.ExitCode = &exit
				return row, nil
			},
		},
		{
			name: "event",
			sql: `SELECT ts, event_kind, tool, target, exit_code, duration_ms, COALESCE(session_id,'')
			      FROM agent_events
			      WHERE repo_id = ? AND agent_id = ?` + tsClause,
			scan: func(rows *sql.Rows, ts int64) (timelineRow, error) {
				row := timelineRow{Source: "event", AgentID: agentID, Kind: "event"}
				var exit int
				if err := rows.Scan(&row.TS, &row.EventKind, &row.Tool, &row.Target, &exit, &row.DurationMS, &row.SessionID); err != nil {
					return row, err
				}
				row.ExitCode = &exit
				return row, nil
			},
		},
	}

	order := "DESC"
	if hasSince {
		order = "ASC"
	}
	for _, q := range queries {
		args := append([]any{s.cfg.RepoID, agentID}, tsArgs...)
		args = append(args, limit)
		rows, err := s.db.QueryContext(ctx, q.sql+" ORDER BY 1 "+order+" LIMIT ?", args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			row, err := q.scan(rows, 0)
			if err != nil {
				rows.Close()
				return nil, err
			}
			all = append(all, row)
		}
		rows.Close()
	}

	sortTimeline(all, hasSince)
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all, nil
}

func sortTimeline(rows []timelineRow, ascending bool) {
	if ascending {
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].TS != rows[j].TS {
				return rows[i].TS < rows[j].TS
			}
			return rows[i].Source < rows[j].Source
		})
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].TS != rows[j].TS {
			return rows[i].TS > rows[j].TS
		}
		return rows[i].Source < rows[j].Source
	})
}

// replaceTSClause swaps the bare `ts >= ?` for a column-aliased version when
// the table's timestamp column isn't called `ts`. Cheap because tsClause is
// either "" or " AND ts >= ?" — no real parsing needed.
func replaceTSClause(clause, col string) string {
	if clause == "" {
		return ""
	}
	return " AND " + col + " >= ?"
}

// nextCursorTS returns a cursor only when paired with `?since=` (ASC mode),
// where the cursor is the newest TS seen and round-trips correctly via
// `?since=cursor` for forward polling. The DESC initial fetch returns null:
// re-issuing `?since=oldest_ts` would re-include the page and everything
// newer, not the next-older window — confusing and unused.
func nextCursorTS(events []agentEventRow, hasSince bool) any {
	if len(events) == 0 || !hasSince {
		return nil
	}
	return events[len(events)-1].TS
}

func parseSince(r *http.Request) (int64, bool, error) {
	raw := r.URL.Query().Get("since")
	if raw == "" {
		return 0, false, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, errors.New("since must be an integer unix timestamp")
	}
	if v < 0 {
		return 0, false, errors.New("since must be non-negative")
	}
	return v, true, nil
}

func parseLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultEventsLimit, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, errors.New("limit must be an integer")
	}
	if v < 1 || v > maxEventsLimit {
		return 0, errors.New("limit must be between 1 and 500")
	}
	return v, nil
}

func (s *Server) agentExists(ctx context.Context, id string) bool {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM agents WHERE id = ?`, id).Scan(&n); err != nil {
		return false
	}
	return n > 0
}
