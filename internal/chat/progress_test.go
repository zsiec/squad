package chat

import (
	"context"
	"testing"
)

func TestReportProgress_StoresRow(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.ReportProgress(ctx, "agent-a", "BUG-9", 50, "halfway"); err != nil {
		t.Fatal(err)
	}

	var pct int
	var note string
	_ = db.QueryRow(`SELECT pct, note FROM progress WHERE item_id = 'BUG-9'`).Scan(&pct, &note)
	if pct != 50 || note != "halfway" {
		t.Fatalf("pct=%d note=%q", pct, note)
	}
}

func TestReportProgress_RejectsOutOfRange(t *testing.T) {
	c, _ := newTestChat(t)
	ctx := context.Background()
	if err := c.ReportProgress(ctx, "agent-a", "BUG-9", -5, ""); err == nil {
		t.Fatal("expected error for pct=-5")
	}
	if err := c.ReportProgress(ctx, "agent-a", "BUG-9", 101, ""); err == nil {
		t.Fatal("expected error for pct=101")
	}
}

func TestProgress_BumpsClaimLastTouch(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if _, err := db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES ('repo-test', 'BUG-700', 'agent-a', 100, 100, '', 0)
	`); err != nil {
		t.Fatal(err)
	}

	if err := c.ReportProgress(ctx, "agent-a", "BUG-700", 50, "still working on it"); err != nil {
		t.Fatal(err)
	}

	var lt int64
	_ = db.QueryRow(`SELECT last_touch FROM claims WHERE item_id='BUG-700'`).Scan(&lt)
	want := c.nowUnix()
	if lt != want {
		t.Fatalf("last_touch=%d want %d", lt, want)
	}
}

func TestLatestProgress_ReturnsNewestRow(t *testing.T) {
	c, _ := newTestChat(t)
	ctx := context.Background()
	_ = c.ReportProgress(ctx, "agent-a", "BUG-9", 25, "first")
	_ = c.ReportProgress(ctx, "agent-a", "BUG-9", 75, "second")
	pct, note := c.LatestProgress(ctx, "BUG-9")
	if pct != 75 || note != "second" {
		t.Fatalf("pct=%d note=%q", pct, note)
	}
}

// If the second of the three writes errors, the first INSERT must roll
// back too — otherwise the stale-claim sweeper sees a progress row with
// a fresh reported_at while claims.last_touch lags, and decides the
// claim is abandoned even though the agent is actively reporting.
func TestReportProgress_RollsBackPartialOnSecondWriteFailure(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if _, err := db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES ('repo-test', 'BUG-700', 'agent-a', 100, 100, '', 0)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER abort_agents_update BEFORE UPDATE ON agents
		FOR EACH ROW BEGIN SELECT RAISE(ABORT, 'forced'); END
	`); err != nil {
		t.Fatal(err)
	}

	if err := c.ReportProgress(ctx, "agent-a", "BUG-700", 50, "should not commit"); err == nil {
		t.Fatal("expected error from forced trigger abort, got nil")
	}

	var progressRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM progress WHERE item_id='BUG-700'`).Scan(&progressRows); err != nil {
		t.Fatal(err)
	}
	if progressRows != 0 {
		t.Fatalf("partial commit: %d progress rows survived; expected 0 (entire transaction must roll back)", progressRows)
	}

	var lastTouch int64
	if err := db.QueryRow(`SELECT last_touch FROM claims WHERE item_id='BUG-700'`).Scan(&lastTouch); err != nil {
		t.Fatal(err)
	}
	if lastTouch != 100 {
		t.Fatalf("claims.last_touch advanced to %d on a failed transaction; expected unchanged 100", lastTouch)
	}
}

// The bus event must only fire after the writes have committed.
// Otherwise a downstream subscriber sees a "progress" event for state
// that never reached disk.
func TestReportProgress_DoesNotPublishOnFailure(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if _, err := db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES ('repo-test', 'BUG-701', 'agent-a', 100, 100, '', 0)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER abort_agents_update_pub BEFORE UPDATE ON agents
		FOR EACH ROW BEGIN SELECT RAISE(ABORT, 'forced'); END
	`); err != nil {
		t.Fatal(err)
	}

	sub := c.Bus().Subscribe()
	defer c.Bus().Unsubscribe(sub)

	if err := c.ReportProgress(ctx, "agent-a", "BUG-701", 50, "should not publish"); err == nil {
		t.Fatal("expected error from forced trigger abort, got nil")
	}

	select {
	case ev := <-sub:
		t.Fatalf("bus published %q event on a failed transaction; payload=%v", ev.Kind, ev.Payload)
	default:
	}
}

// CHORE-005 regression: a failure between progress-row insert and
// messages-row insert must roll both back. Previously PostProgress ran
// outside the transaction; a partial failure left progress and messages
// disagreeing on whether the report happened.
func TestReportProgress_AtomicWithMessageInsert(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if _, err := db.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES ('repo-test', 'BUG-707', 'agent-a', 100, 100, '', 0)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER abort_messages_insert BEFORE INSERT ON messages
		FOR EACH ROW BEGIN SELECT RAISE(ABORT, 'forced'); END
	`); err != nil {
		t.Fatal(err)
	}

	if err := c.ReportProgress(ctx, "agent-a", "BUG-707", 50, "halfway"); err == nil {
		t.Fatal("expected error from forced trigger abort, got nil")
	}

	var progressRows int
	_ = db.QueryRow(`SELECT COUNT(*) FROM progress WHERE item_id='BUG-707'`).Scan(&progressRows)
	var msgRows int
	_ = db.QueryRow(`SELECT COUNT(*) FROM messages WHERE thread='BUG-707' AND kind='progress'`).Scan(&msgRows)
	if progressRows != 0 || msgRows != 0 {
		t.Fatalf("partial commit: progress=%d messages=%d (both should be 0)", progressRows, msgRows)
	}
}

// Happy path: a single ReportProgress writes both the progress row and
// the chat message in one transaction — the SSE pump and the LatestProgress
// reader cannot disagree.
func TestReportProgress_WritesBothProgressAndMessage(t *testing.T) {
	c, db := newTestChat(t)
	ctx := context.Background()

	if err := c.ReportProgress(ctx, "agent-a", "BUG-708", 25, "first quarter"); err != nil {
		t.Fatal(err)
	}

	var progressRows int
	_ = db.QueryRow(`SELECT COUNT(*) FROM progress WHERE item_id='BUG-708'`).Scan(&progressRows)
	if progressRows != 1 {
		t.Errorf("progress rows=%d want 1", progressRows)
	}
	var body string
	_ = db.QueryRow(`SELECT body FROM messages WHERE thread='BUG-708' AND kind='progress'`).Scan(&body)
	if body != "25% — first quarter" {
		t.Errorf("message body=%q want %q", body, "25% — first quarter")
	}
}
