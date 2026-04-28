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
	HasMention  bool   // peer's thread carries an unread @<me> mention
}

// printPeerDigest writes the standup block at squad-go time. The agent
// who just claimed an item gets a one-screen view of who else is
// active, in which area, and the latest human-typed thing they said.
// On `area` overlap with one or more peers, an inline nudge precedes
// the AC suggesting an `squad ask @<peer>` ack.
//
// Peers whose threads carry an unread `@<myAgentID>` mention surface
// above the last-touch ordering with a `*` marker — the mailbox flush
// tells the same story but the digest is the one screen agents are
// guaranteed to read at session start.
//
// myAgentID and myItemID are filtered out of the rendered list so the
// digest is about *peers*, not the calling session. myArea drives the
// overlap-nudge; pass empty when no area is set on the new claim.
func printPeerDigest(ctx context.Context, db *sql.DB, repoID, myAgentID, myItemID, myArea string, w io.Writer, now time.Time) error {
	rows, err := loadPeerRows(ctx, db, repoID, myAgentID, myItemID)
	if err != nil {
		return err
	}
	rows, err = annotateMentions(ctx, db, repoID, myAgentID, rows)
	if err != nil {
		return err
	}
	renderPeerDigest(w, sortByMentionThenLastTouch(rows), myArea, now)
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
// peer is up to. Sourced from squad-chat-cadence's verb taxonomy.
// Excludes `done` / `progress` / `handoff` / `system` / `review_req`
// because those are framework-emitted or otherwise structural rather
// than a peer status update.
var humanVerbKinds = []string{"thinking", "milestone", "stuck", "fyi", "ask", "say"}

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
		marker := " "
		if p.HasMention {
			marker = "*"
		}
		fmt.Fprintf(w, "  %s @%s on %s (area=%s, %s) %s\n",
			marker, displayHandle(p), p.ItemID, p.Area, age, excerpt)
	}
	if overflow > 0 {
		fmt.Fprintf(w, "  … (+%d more)\n", overflow)
	}
	fmt.Fprintln(w)
}

// annotateMentions sets HasMention=true on each peer row whose thread
// has an unread message (relative to my reads.last_msg_id) that mentions
// my agent id. Mention detection is body-based for parity with the
// existing chat digest path (`@<agentID>` substring) — that's what the
// mailbox flush surfaces, so the digest mirrors the same signal.
func annotateMentions(ctx context.Context, db *sql.DB, repoID, myAgentID string, peers []peerRow) ([]peerRow, error) {
	if len(peers) == 0 {
		return peers, nil
	}
	q := `
		SELECT 1
		FROM messages m
		LEFT JOIN reads r ON r.agent_id = ? AND r.thread = m.thread
		WHERE m.repo_id = ?
		  AND m.thread = ?
		  AND m.id > COALESCE(r.last_msg_id, 0)
		  AND m.agent_id != ?
		  AND m.body LIKE ?
		LIMIT 1
	`
	mentionTag := "%@" + myAgentID + "%"
	for i := range peers {
		var one int
		err := db.QueryRowContext(ctx, q, myAgentID, repoID, peers[i].ItemID, myAgentID, mentionTag).Scan(&one)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		peers[i].HasMention = true
	}
	return peers, nil
}

// sortByMentionThenLastTouch partitions peers into two stable groups —
// those with unread mentions of the calling agent, and those without —
// and concatenates them (mention group first). Within each group the
// caller's last_touch DESC ordering is preserved.
func sortByMentionThenLastTouch(peers []peerRow) []peerRow {
	if len(peers) == 0 {
		return peers
	}
	mentioned := make([]peerRow, 0, len(peers))
	rest := make([]peerRow, 0, len(peers))
	for _, p := range peers {
		if p.HasMention {
			mentioned = append(mentioned, p)
		} else {
			rest = append(rest, p)
		}
	}
	return append(mentioned, rest...)
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
