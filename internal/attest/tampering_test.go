package attest_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/store"
)

func TestTampering_DetectedAcrossFullPath(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	L := attest.New(db, "repo-x", nil)

	rec, err := L.Run(context.Background(), attest.RunOpts{
		ItemID:   "FEAT-001",
		Kind:     attest.KindTest,
		Command:  `printf 'PASS\nok ./...\n'`,
		AgentID:  "agent-a",
		AttDir:   filepath.Join(dir, ".squad", "attestations"),
		RepoRoot: dir,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	originalHash := rec.OutputHash
	if err := L.Verify(context.Background(), "FEAT-001"); err != nil {
		t.Fatalf("clean ledger should verify, got %v", err)
	}

	// Adversarially modify the file on disk: a malicious agent erases a FAIL
	// summary and replaces it with PASS. The hash must catch this.
	if err := os.WriteFile(rec.OutputPath, []byte("PASS\nok (mutated)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verr := L.Verify(context.Background(), "FEAT-001")
	if verr == nil {
		t.Fatal("post-mutation Verify should fail")
	}
	if !strings.Contains(verr.Error(), "hash mismatch") {
		t.Fatalf("err = %v, want hash mismatch", verr)
	}
	if !strings.Contains(verr.Error(), originalHash) {
		t.Fatalf("err should reference recorded hash %q, got %v", originalHash, verr)
	}
}

func TestTampering_DeletionDetected(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	L := attest.New(db, "repo-x", nil)

	rec, err := L.Run(context.Background(), attest.RunOpts{
		ItemID:   "FEAT-001",
		Kind:     attest.KindTest,
		Command:  `printf 'ok\n'`,
		AgentID:  "a",
		AttDir:   filepath.Join(dir, ".squad", "attestations"),
		RepoRoot: dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rec.OutputPath); err != nil {
		t.Fatal(err)
	}
	verr := L.Verify(context.Background(), "FEAT-001")
	if verr == nil {
		t.Fatal("expected verify to fail after file deletion")
	}
	if !strings.Contains(verr.Error(), "missing") {
		t.Fatalf("err = %v, want missing-file", verr)
	}
}
