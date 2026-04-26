package hygiene

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func insertItem(t *testing.T, db *sql.DB, repoID, itemID, status string, capturedAt *int64) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO items (repo_id, item_id, title, type, priority, status, path, updated_at, captured_at)
		VALUES (?, ?, ?, 'feat', 'P3', ?, ?, ?, ?)
	`, repoID, itemID, itemID+" title", status, ".squad/items/"+itemID+".md", time.Now().Unix(), capturedAt)
	if err != nil {
		t.Fatalf("insert item %s: %v", itemID, err)
	}
}

func TestCheckStaleCaptures_FlagsOldItems(t *testing.T) {
	db := newDB(t)
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	young := time.Now().Add(-5 * 24 * time.Hour).Unix()
	insertItem(t, db, "repo-test", "FEAT-OLD", "captured", &old)
	insertItem(t, db, "repo-test", "FEAT-YOUNG", "captured", &young)

	findings := CheckStaleCaptures(context.Background(), db, "repo-test", 30*24*time.Hour)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "FEAT-OLD") {
		t.Fatalf("finding does not mention FEAT-OLD: %s", findings[0].Message)
	}
	if findings[0].Code != "stale_capture" {
		t.Fatalf("want code stale_capture, got %s", findings[0].Code)
	}
}

func TestCheckStaleCaptures_NoStaleReturnsEmpty(t *testing.T) {
	db := newDB(t)
	young := time.Now().Add(-1 * 24 * time.Hour).Unix()
	insertItem(t, db, "repo-test", "FEAT-FRESH", "captured", &young)

	findings := CheckStaleCaptures(context.Background(), db, "repo-test", 30*24*time.Hour)
	if len(findings) != 0 {
		t.Fatalf("want 0 findings, got %d: %v", len(findings), findings)
	}
}

func TestCheckInboxOverflow_FlagsOversizedInbox(t *testing.T) {
	db := newDB(t)
	now := time.Now().Unix()
	for i := 0; i < 51; i++ {
		insertItem(t, db, "repo-test", fmt.Sprintf("FEAT-%03d", i), "captured", &now)
	}

	findings := CheckInboxOverflow(context.Background(), db, "repo-test", 50)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "51") {
		t.Fatalf("finding missing count 51: %s", findings[0].Message)
	}
	if findings[0].Code != "inbox_overflow" {
		t.Fatalf("want code inbox_overflow, got %s", findings[0].Code)
	}
}

func TestCheckInboxOverflow_AtThresholdNoFinding(t *testing.T) {
	db := newDB(t)
	now := time.Now().Unix()
	for i := 0; i < 50; i++ {
		insertItem(t, db, "repo-test", fmt.Sprintf("FEAT-%03d", i), "captured", &now)
	}

	findings := CheckInboxOverflow(context.Background(), db, "repo-test", 50)
	if len(findings) != 0 {
		t.Fatalf("want 0 findings at threshold, got %d: %v", len(findings), findings)
	}
}

func TestCheckRejectedLogSize_FlagsLargeLog(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 0; i < 501; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "rejected.log"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	findings := CheckRejectedLogSize(dir, 500)
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "501") {
		t.Fatalf("finding missing count 501: %s", findings[0].Message)
	}
	if findings[0].Code != "rejected_log_overflow" {
		t.Fatalf("want code rejected_log_overflow, got %s", findings[0].Code)
	}
}

func TestCheckRejectedLogSize_MissingLogReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	findings := CheckRejectedLogSize(dir, 500)
	if len(findings) != 0 {
		t.Fatalf("want 0 findings for missing log, got %d: %v", len(findings), findings)
	}
}

func TestCheckRejectedLogSize_AtThresholdNoFinding(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, "rejected.log"), []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	findings := CheckRejectedLogSize(dir, 500)
	if len(findings) != 0 {
		t.Fatalf("want 0 findings at threshold, got %d: %v", len(findings), findings)
	}
}
