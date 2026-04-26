package attest

import (
	"context"
	"errors"
	"fmt"
	"os"
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

func TestLedger_Verify_DetectsTampering(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)

	payload := []byte("PASS\nok  \tgithub.com/x/y\t0.123s\n")
	hash := L.Hash(payload)
	outPath := filepath.Join(dir, "att", hash+".txt")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	rec := Record{
		ItemID: "FEAT-001", Kind: KindTest, Command: "go test ./...",
		OutputHash: hash, OutputPath: outPath, AgentID: "a",
	}
	if _, err := L.Insert(context.Background(), rec); err != nil {
		t.Fatal(err)
	}

	if err := L.Verify(context.Background(), "FEAT-001"); err != nil {
		t.Fatalf("clean ledger should verify, got %v", err)
	}

	if err := os.WriteFile(outPath, []byte("FAKED PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = L.Verify(context.Background(), "FEAT-001")
	if err == nil {
		t.Fatal("expected tampering error")
	}
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("want ErrHashMismatch, got %v", err)
	}
}

func TestLedger_MissingKinds(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)
	add := func(kind Kind, exit int) {
		t.Helper()
		_, err := L.Insert(context.Background(), Record{
			ItemID: "FEAT-001", Kind: kind, Command: string(kind),
			OutputHash: fmt.Sprintf("%s-hash-%d", kind, exit), OutputPath: "p", ExitCode: exit, AgentID: "a",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	add(KindTest, 0)
	missing, err := L.MissingKinds(context.Background(), "FEAT-001", []Kind{KindTest, KindReview})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != KindReview {
		t.Fatalf("missing = %v, want [review]", missing)
	}

	// A failing test attestation does NOT satisfy the requirement.
	add(KindReview, 1)
	missing, err = L.MissingKinds(context.Background(), "FEAT-001", []Kind{KindTest, KindReview})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != KindReview {
		t.Fatalf("after failing review: missing = %v, want [review]", missing)
	}

	// A passing review row replaces it.
	add(KindReview, 0)
	missing, err = L.MissingKinds(context.Background(), "FEAT-001", []Kind{KindTest, KindReview})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("with both passing: missing = %v, want []", missing)
	}
}

func TestLedger_Verify_MissingFile(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)
	rec := Record{
		ItemID: "FEAT-001", Kind: KindTest, Command: "x",
		OutputHash: "h", OutputPath: filepath.Join(dir, "missing.txt"),
		AgentID: "a",
	}
	if _, err := L.Insert(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	err = L.Verify(context.Background(), "FEAT-001")
	if !errors.Is(err, ErrOutputMissing) {
		t.Fatalf("want ErrOutputMissing, got %v", err)
	}
}
