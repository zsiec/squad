package attest

import (
	"context"
	"path/filepath"
	"testing"

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
