package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"
)

// peerDigestCap is the maximum number of peer rows the standup digest
// renders before collapsing the rest into a `… (+N more)` line. Six is
// roughly what fits on a typical terminal pane between the claim line
// and the AC block without scrolling the AC out of view.
const peerDigestCap = 6

// peerDigestExcerptLen caps the per-row chat-excerpt length. Plenty of
// signal in 80 chars; longer excerpts push the area column off-screen
// on narrow terminals.
const peerDigestExcerptLen = 80

// peerRow is the rendered shape for one active peer claim. The fields
// match the rendered columns; see printPeerDigest for the layout.
type peerRow struct {
	AgentID     string
	DisplayName string
	ItemID      string
	Area        string
	LastTouch   int64
	Excerpt     string // most recent human-verb post body, truncated
}

// printPeerDigest writes the standup block at squad-go time. The agent
// who just claimed an item gets a one-screen view of who else is
// active, in which area, and the latest human-typed thing they said.
// On `area` overlap with one or more peers, an inline nudge precedes
// the AC suggesting an `squad ask @<peer>` ack.
//
// myAgentID and myItemID are filtered out of the rendered list so the
// digest is about *peers*, not the calling session. myArea drives the
// overlap-nudge; pass empty when no area is set on the new claim.
func printPeerDigest(ctx context.Context, db *sql.DB, repoID, myAgentID, myItemID, myArea string, w io.Writer, now time.Time) error {
	rows, err := loadPeerRows(ctx, db, repoID, myAgentID, myItemID)
	if err != nil {
		return err
	}
	renderPeerDigest(w, rows, myArea, now)
	return nil
}

// loadPeerRows pulls active claims (excluding the caller's), joins
// agents for display_name + items for area, and attaches the most
// recent human-verb chat excerpt per item. Returns the rows sorted
// last_touch DESC. Excerpts are only fetched for the first
// peerDigestCap rows so a busy repo doesn't pay N+1 SQLite roundtrips
// to render six lines; rows beyond the cap have empty Excerpt and
// surface only as the `+N more` overflow line in the renderer.
func loadPeerRows(ctx context.Context, db *sql.DB, repoID, myAgentID, myItemID string) ([]peerRow, error) {
	q := `
		SELECT c.agent_id,
		       COALESCE(a.display_name, ''),
		       c.item_id,
		       COALESCE(i.area, ''),
		       c.last_touch
		FROM claims c
		LEFT JOIN agents a ON a.id = c.agent_id
		LEFT JOIN items  i ON i.repo_id = c.repo_id AND i.item_id = c.item_id
		WHERE c.repo_id = ? AND c.agent_id != ? AND c.item_id != ?
		ORDER BY c.last_touch DESC
	`
	out, err := db.QueryContext(ctx, q, repoID, myAgentID, myItemID)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	var peers []peerRow
	for out.Next() {
		var p peerRow
		if err := out.Scan(&p.AgentID, &p.DisplayName, &p.ItemID, &p.Area, &p.LastTouch); err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	if err := out.Err(); err != nil {
		return nil, err
	}
	excerptLimit := peerDigestCap
	if excerptLimit > len(peers) {
		excerptLimit = len(peers)
	}
	for i := 0; i < excerptLimit; i++ {
		excerpt, err := latestHumanVerbExcerpt(ctx, db, repoID, peers[i].ItemID)
		if err != nil {
			return nil, err
		}
		peers[i].Excerpt = excerpt
	}
	return peers, nil
}

// humanVerbKinds is the set of chat verbs the digest considers
// "human-typed" — surfaceable as a meaningful excerpt of what the
// peer is up to. Sourced from squad-chat-cadence's verb taxonomy plus
// `answer` (a directed reply that's still human-authored). Excludes
// `done` / `progress` / `handoff` / `knock` / `system` / `review_req`
// because those are framework-emitted or otherwise structural rather
// than a peer status update.
var humanVerbKinds = []string{"thinking", "milestone", "stuck", "fyi", "ask", "say", "answer"}

func latestHumanVerbExcerpt(ctx context.Context, db *sql.DB, repoID, itemID string) (string, error) {
	placeholders := strings.Repeat("?,", len(humanVerbKinds))
	placeholders = strings.TrimRight(placeholders, ",")
	q := `
		SELECT COALESCE(body, ''), COALESCE(kind, '')
		FROM messages
		WHERE repo_id = ? AND thread = ? AND kind IN (` + placeholders + `)
		ORDER BY ts DESC LIMIT 1
	`
	args := []any{repoID, itemID}
	for _, k := range humanVerbKinds {
		args = append(args, k)
	}
	var body, kind string
	row := db.QueryRowContext(ctx, q, args...)
	if err := row.Scan(&body, &kind); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return formatExcerpt(kind, body), nil
}

// formatExcerpt prefixes the chat body with its verb tag and truncates
// to the per-row cap. Empty body is rendered as the bare verb so peers
// who post zero-body messages (rare but legal) still surface.
func formatExcerpt(kind, body string) string {
	body = strings.TrimSpace(body)
	body = strings.ReplaceAll(body, "\n", " ")
	if kind == "" {
		return ""
	}
	out := kind + ": " + body
	if len(out) > peerDigestExcerptLen {
		out = out[:peerDigestExcerptLen-1] + "…"
	}
	return out
}

func renderPeerDigest(w io.Writer, peers []peerRow, myArea string, now time.Time) {
	if len(peers) == 0 {
		fmt.Fprintln(w, "peers: none active.")
		fmt.Fprintln(w)
		return
	}
	overlap := overlappingPeers(peers, myArea)
	if len(overlap) > 0 {
		// Nudge surfaces the primary peer; multi-peer overlap is
		// surfaced as the rendered list still shows them all.
		first := overlap[0]
		fmt.Fprintf(w, "overlaps with @%s on %s (area=%s) — ack with 'squad ask @%s ...' before starting\n",
			displayHandle(first), first.ItemID, first.Area, displayHandle(first))
	}
	fmt.Fprintln(w, "peers:")
	visible := peers
	overflow := 0
	if len(visible) > peerDigestCap {
		overflow = len(visible) - peerDigestCap
		visible = visible[:peerDigestCap]
	}
	for _, p := range visible {
		age := humanizeAge(now, p.LastTouch)
		excerpt := p.Excerpt
		if excerpt == "" {
			excerpt = "(no posts yet)"
		}
		fmt.Fprintf(w, "  @%s on %s (area=%s, %s) %s\n",
			displayHandle(p), p.ItemID, p.Area, age, excerpt)
	}
	if overflow > 0 {
		fmt.Fprintf(w, "  … (+%d more)\n", overflow)
	}
	fmt.Fprintln(w)
}

func overlappingPeers(peers []peerRow, myArea string) []peerRow {
	if strings.TrimSpace(myArea) == "" {
		return nil
	}
	var out []peerRow
	for _, p := range peers {
		if strings.EqualFold(strings.TrimSpace(p.Area), strings.TrimSpace(myArea)) {
			out = append(out, p)
		}
	}
	return out
}

func displayHandle(p peerRow) string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.AgentID
}

// humanizeAge renders a unix-second delta as "2h", "15m", "30s", etc.
// For never-touched rows (last_touch == 0) returns "never".
func humanizeAge(now time.Time, lastTouch int64) string {
	if lastTouch == 0 {
		return "never"
	}
	d := now.Sub(time.Unix(lastTouch, 0))
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
