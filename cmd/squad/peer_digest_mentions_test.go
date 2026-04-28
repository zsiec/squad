package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

// insertMentionMsg inserts a message in `peerItemID`'s thread that
// mentions `myAgentID` in the body. Caller must seed the peer separately.
// last_msg_id in `reads` for `myAgentID` is left at 0 (or whatever the
// caller set), so the message counts as unread.
func insertMentionMsg(t *testing.T, db *sql.DB, repoID, fromAgent, peerItemID, body string, ts int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body, mentions, priority)
		VALUES (?, ?, ?, ?, 'ask', ?, '', 'normal')
	`, repoID, ts, fromAgent, peerItemID, body); err != nil {
		t.Fatalf("insert mention message: %v", err)
	}
}

// markThreadRead pins `reads.last_msg_id` for (myAgent, thread) so any
// further inserts go to "unread" automatically.
func markThreadRead(t *testing.T, db *sql.DB, myAgent, thread string, lastMsgID int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO reads (agent_id, thread, last_msg_id)
		VALUES (?, ?, ?)
		ON CONFLICT(agent_id, thread) DO UPDATE SET last_msg_id=excluded.last_msg_id
	`, myAgent, thread, lastMsgID); err != nil {
		t.Fatalf("mark read: %v", err)
	}
}

// TestPeerDigest_MentionedPeerSurfacesFirstWithMarker pins the contract:
// when a peer's thread contains an unread message mentioning the calling
// agent, that peer is rendered above peers without mentions, regardless
// of last_touch order, with a visible marker.
func TestPeerDigest_MentionedPeerSurfacesFirstWithMarker(t *testing.T) {
	db, repoID, now := peerDigestFixture(t)
	// Two peers. The "no-mention" peer was touched MORE recently than
	// the "mention" peer — so without the new ordering rule, "no-mention"
	// would render first.
	insertPeer(t, db, repoID, "agent-asker", "Asker", "BUG-201", "auth", now.Add(-10*time.Minute).Unix())
	insertPeer(t, db, repoID, "agent-quiet", "Quiet", "BUG-202", "intake", now.Add(-1*time.Minute).Unix())
	// Mention message in Asker's thread; no read marker, so it counts unread.
	insertMentionMsg(t, db, repoID, "agent-asker", "BUG-201",
		"@agent-me thoughts on this approach?", now.Add(-9*time.Minute).Unix())
	// And a regular thinking post in Quiet's thread for the excerpt.
	insertPeerMessage(t, db, repoID, "agent-quiet", "BUG-202", "thinking", "weighing options", now.Add(-1*time.Minute).Unix())

	var buf bytes.Buffer
	if err := printPeerDigest(context.Background(), db, repoID, "agent-me", "BUG-MINE", "", &buf, now); err != nil {
		t.Fatal(err)
	}
	got := buf.String()

	idxAsker := strings.Index(got, "BUG-201")
	idxQuiet := strings.Index(got, "BUG-202")
	if idxAsker < 0 || idxQuiet < 0 {
		t.Fatalf("both peers should appear; got %q", got)
	}
	if idxAsker > idxQuiet {
		t.Errorf("mentioned peer (BUG-201) should render before non-mention peer (BUG-202); got order Quiet→Asker:\n%s", got)
	}
	// Marker present on the mentioned row; locate the BUG-201 line and
	// check it carries a '*' marker (or '@' for mention) as the new
	// prefix the implementation chooses.
	bug201Line := lineContaining(got, "BUG-201")
	if !strings.Contains(bug201Line, "*") {
		t.Errorf("mentioned peer row should carry a '*' marker; got line %q", bug201Line)
	}
	bug202Line := lineContaining(got, "BUG-202")
	if strings.Contains(bug202Line, "*") {
		t.Errorf("non-mention peer row should not carry the '*' marker; got line %q", bug202Line)
	}
}

// TestPeerDigest_SevenPeersThreeMentionsLandAtTop pins both the
// prioritized-mentions ordering and the cap-of-six truncation
// interacting correctly: three mentioned peers must all appear among
// the rendered six, the cap drops a non-mention row first.
func TestPeerDigest_SevenPeersThreeMentionsLandAtTop(t *testing.T) {
	db, repoID, now := peerDigestFixture(t)
	// Seven peers. Last 3 (oldest by last_touch) are the ones with
	// unread mentions — so plain last_touch order would push them off
	// the cap. Mention prioritization should pull them to the top.
	for i := 0; i < 7; i++ {
		ts := now.Add(time.Duration(-i) * time.Minute).Unix()
		insertPeer(t, db, repoID,
			fmt.Sprintf("agent-p%d", i),
			fmt.Sprintf("Peer%d", i),
			fmt.Sprintf("BUG-30%d", i),
			fmt.Sprintf("area-%d", i),
			ts)
	}
	mentionFor := []int{4, 5, 6}
	for _, i := range mentionFor {
		insertMentionMsg(t, db, repoID,
			fmt.Sprintf("agent-p%d", i),
			fmt.Sprintf("BUG-30%d", i),
			fmt.Sprintf("@agent-me ping on BUG-30%d", i),
			now.Add(time.Duration(-i)*time.Minute+time.Second).Unix())
	}

	var buf bytes.Buffer
	if err := printPeerDigest(context.Background(), db, repoID, "agent-me", "BUG-MINE", "", &buf, now); err != nil {
		t.Fatal(err)
	}
	got := buf.String()

	for _, i := range mentionFor {
		want := fmt.Sprintf("BUG-30%d", i)
		if !strings.Contains(got, want) {
			t.Errorf("mentioned peer %s missing from rendered digest:\n%s", want, got)
		}
	}
	// Cap rule still holds.
	rows := strings.Count(got, "\n  ")
	// 6 peer rows + 1 truncation line "… (+1 more)".
	if rows != peerDigestCap+1 {
		t.Errorf("expected %d rendered lines (6 peer rows + truncation); got %d:\n%s",
			peerDigestCap+1, rows, got)
	}
	if !strings.Contains(got, "(+1 more)") {
		t.Errorf("seven peers should still truncate to '+1 more'; got %q", got)
	}
	// The three mention rows should each carry the marker.
	for _, i := range mentionFor {
		line := lineContaining(got, fmt.Sprintf("BUG-30%d", i))
		if !strings.Contains(line, "*") {
			t.Errorf("mentioned peer BUG-30%d row missing '*' marker: %q", i, line)
		}
	}
}

// TestPeerDigest_ReadMentionDoesNotPrioritize pins that a mention the
// caller has already seen (last_msg_id past the mention's id) does NOT
// trigger prioritization. The unread-bound is load-bearing — without it,
// every old @<me> would re-surface forever.
func TestPeerDigest_ReadMentionDoesNotPrioritize(t *testing.T) {
	db, repoID, now := peerDigestFixture(t)
	insertPeer(t, db, repoID, "agent-old", "Old", "BUG-401", "auth", now.Add(-10*time.Minute).Unix())
	insertPeer(t, db, repoID, "agent-new", "New", "BUG-402", "intake", now.Add(-1*time.Minute).Unix())
	insertMentionMsg(t, db, repoID, "agent-old", "BUG-401",
		"@agent-me historical question", now.Add(-9*time.Minute).Unix())
	// Mark Old's thread as read past the mention. New post id is 1
	// (only one message inserted); set last_msg_id ≥ 1.
	markThreadRead(t, db, "agent-me", "BUG-401", 999)

	var buf bytes.Buffer
	if err := printPeerDigest(context.Background(), db, repoID, "agent-me", "BUG-MINE", "", &buf, now); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	// Without unread mentions, last_touch DESC order should put New first.
	idxOld := strings.Index(got, "BUG-401")
	idxNew := strings.Index(got, "BUG-402")
	if idxOld < idxNew {
		t.Errorf("a read mention must not prioritize; expected New (BUG-402) before Old (BUG-401):\n%s", got)
	}
	oldLine := lineContaining(got, "BUG-401")
	if strings.Contains(oldLine, "*") {
		t.Errorf("read mention must not carry the '*' marker:\n%q", oldLine)
	}
}

func lineContaining(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
