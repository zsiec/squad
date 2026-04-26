package attest

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func TestNewLedger_HashEmptyBytesIsKnown(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)
	got := L.Hash(nil)
	const sha256OfEmpty = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if got != sha256OfEmpty {
		t.Fatalf("hash(empty) = %q, want %q", got, sha256OfEmpty)
	}
	if _, err := L.ListForItem(context.Background(), "FEAT-001"); err != nil {
		t.Fatalf("ListForItem on empty ledger: %v", err)
	}
}

func TestLedger_Insert_RoundTrip(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	L := New(db, "repo-x", func() time.Time { return time.Unix(1700000000, 0) })

	rec := Record{
		ItemID:     "FEAT-001",
		Kind:       KindTest,
		Command:    "go test ./...",
		ExitCode:   0,
		OutputHash: "deadbeef",
		OutputPath: ".squad/attestations/deadbeef.txt",
		AgentID:    "agent-a",
	}
	id, err := L.Insert(context.Background(), rec)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero rowid")
	}
	got, err := L.ListForItem(context.Background(), "FEAT-001")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].CreatedAt != 1700000000 {
		t.Fatalf("created_at = %d, want 1700000000", got[0].CreatedAt)
	}
	if got[0].RepoID != "repo-x" {
		t.Fatalf("repo_id = %q, want repo-x", got[0].RepoID)
	}
	if got[0].Kind != KindTest {
		t.Fatalf("kind = %q, want test", got[0].Kind)
	}
}

func TestLedger_Insert_InvalidKind(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)
	_, err = L.Insert(context.Background(), Record{
		ItemID: "FEAT-001", Kind: "fabricated", Command: "x",
		OutputHash: "h", OutputPath: "p", AgentID: "a",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid kind") {
		t.Fatalf("expected invalid kind error, got %v", err)
	}
}

func TestLedger_Insert_DuplicateHashDedupes(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)
	rec := Record{
		ItemID: "FEAT-001", Kind: KindTest, Command: "go test ./...",
		OutputHash: "abc", OutputPath: "p", AgentID: "a",
	}
	if _, err := L.Insert(context.Background(), rec); err != nil {
		t.Fatalf("first insert: %v", err)
	}
	id, err := L.Insert(context.Background(), rec)
	if err != nil {
		t.Fatalf("dup insert should be idempotent, got %v", err)
	}
	if id == 0 {
		t.Fatal("expected idempotent insert to return existing rowid")
	}
	got, err := L.ListForItem(context.Background(), "FEAT-001")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1 (dedup)", len(got))
	}
}
