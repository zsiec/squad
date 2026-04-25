package prmark

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPendingRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pending-prs.json")

	now := time.Unix(1_700_000_000, 0).UTC()

	if err := AppendPending(path, Entry{ItemID: "BUG-001", Branch: "fix/foo", CreatedAt: now}); err != nil {
		t.Fatalf("AppendPending: %v", err)
	}
	if err := AppendPending(path, Entry{ItemID: "FEAT-002", Branch: "feat/bar", CreatedAt: now.Add(time.Minute)}); err != nil {
		t.Fatalf("AppendPending #2: %v", err)
	}

	got, err := ReadPending(path)
	if err != nil {
		t.Fatalf("ReadPending: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ItemID != "BUG-001" || got[0].Branch != "fix/foo" {
		t.Fatalf("entry 0 = %+v", got[0])
	}
	if got[1].ItemID != "FEAT-002" || got[1].Branch != "feat/bar" {
		t.Fatalf("entry 1 = %+v", got[1])
	}

	if err := RemovePending(path, "BUG-001"); err != nil {
		t.Fatalf("RemovePending: %v", err)
	}
	got, err = ReadPending(path)
	if err != nil {
		t.Fatalf("ReadPending after remove: %v", err)
	}
	if len(got) != 1 || got[0].ItemID != "FEAT-002" {
		t.Fatalf("after remove, got %+v", got)
	}
}

func TestRemovePendingMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "absent.json")
	if err := RemovePending(path, "X-1"); err != nil {
		t.Fatalf("RemovePending on missing file: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("RemovePending must not create the file; stat err = %v", err)
	}
}
