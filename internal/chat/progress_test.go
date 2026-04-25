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
