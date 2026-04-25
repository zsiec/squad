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
