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

func TestRun_CapturesOutputAndHashes(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)

	rec, err := L.Run(context.Background(), RunOpts{
		ItemID:   "FEAT-001",
		Kind:     KindTest,
		Command:  `printf 'PASS\nok\n'`,
		AgentID:  "agent-a",
		AttDir:   filepath.Join(dir, ".squad", "attestations"),
		RepoRoot: dir,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rec.ExitCode != 0 {
		t.Fatalf("exit = %d, want 0", rec.ExitCode)
	}
	body, err := os.ReadFile(rec.OutputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(body) != "PASS\nok\n" {
		t.Fatalf("body = %q", string(body))
	}
	if rec.OutputHash != L.Hash(body) {
		t.Fatalf("hash mismatch in returned record")
	}

	// Hash-named filename invariant.
	want := filepath.Join(dir, ".squad", "attestations", rec.OutputHash+".txt")
	if rec.OutputPath != want {
		t.Fatalf("OutputPath = %q, want %q", rec.OutputPath, want)
	}
}

func TestRun_NonZeroExitStillRecorded(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)
	rec, err := L.Run(context.Background(), RunOpts{
		ItemID:   "FEAT-001",
		Kind:     KindTest,
		Command:  `echo failing >&2 ; exit 1`,
		AgentID:  "agent-a",
		AttDir:   filepath.Join(dir, "att"),
		RepoRoot: dir,
	})
	if err != nil {
		t.Fatalf("Run should not error on non-zero exit, got %v", err)
	}
	if rec.ExitCode != 1 {
		t.Fatalf("exit = %d, want 1", rec.ExitCode)
	}
	got, err := L.ListForItem(context.Background(), "FEAT-001")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ExitCode != 1 {
		t.Fatalf("expected one row, exit=1, got %+v", got)
	}
}

func TestLedger_Insert_CrossRepoSameHashIsNotDedup(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	A := New(db, "repo-a", nil)
	B := New(db, "repo-b", nil)

	rec := Record{
		ItemID: "FEAT-001", Kind: KindTest, Command: "go test ./...",
		OutputHash: "shared-hash", OutputPath: "p", AgentID: "agent-x",
	}
	idA, err := A.Insert(context.Background(), rec)
	if err != nil {
		t.Fatalf("insert A: %v", err)
	}
	idB, err := B.Insert(context.Background(), rec)
	if err != nil {
		t.Fatalf("insert B: %v (cross-repo same-hash should not collide)", err)
	}
	if idA == idB {
		t.Fatalf("idA == idB == %d (cross-repo collision: dedup returned other repo's row id)", idA)
	}

	aRecs, _ := A.ListForItem(context.Background(), "FEAT-001")
	bRecs, _ := B.ListForItem(context.Background(), "FEAT-001")
	if len(aRecs) != 1 || len(bRecs) != 1 {
		t.Fatalf("repo-a saw %d rows, repo-b saw %d rows; want 1 each", len(aRecs), len(bRecs))
	}
}

func TestLedger_Insert_SameHashDifferentKindIsNotDedup(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	L := New(db, "repo-x", nil)

	base := Record{
		ItemID: "FEAT-001", Command: "x",
		OutputHash: "h", OutputPath: "p", AgentID: "a",
	}
	base.Kind = KindTest
	if _, err := L.Insert(context.Background(), base); err != nil {
		t.Fatalf("insert test: %v", err)
	}
	base.Kind = KindLint
	if _, err := L.Insert(context.Background(), base); err != nil {
		t.Fatalf("insert lint: %v (same hash different kind should not collide)", err)
	}
	recs, _ := L.ListForItem(context.Background(), "FEAT-001")
	if len(recs) != 2 {
		t.Fatalf("len(recs) = %d, want 2", len(recs))
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

// TestLedger_ListForItem_WorkspaceModeAggregatesAllRepos pins the
// "" sentinel: a Ledger constructed with repoID == "" must list rows
// from every repo, not silently filter to repo_id = ''. The dashboard
// daemon takes this path when no repo is discovered at startup.
func TestLedger_ListForItem_WorkspaceModeAggregatesAllRepos(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	mkRec := func(hash string) Record {
		return Record{
			ItemID:     "FEAT-700",
			Kind:       KindTest,
			Command:    "go test ./...",
			ExitCode:   0,
			OutputHash: hash,
			OutputPath: ".squad/attestations/" + hash + ".txt",
			AgentID:    "agent-x",
		}
	}
	if _, err := New(db, "repo-A", nil).Insert(context.Background(), mkRec("aaa")); err != nil {
		t.Fatal(err)
	}
	if _, err := New(db, "repo-B", nil).Insert(context.Background(), mkRec("bbb")); err != nil {
		t.Fatal(err)
	}

	got, err := New(db, "", nil).ListForItem(context.Background(), "FEAT-700")
	if err != nil {
		t.Fatalf("workspace ListForItem: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("workspace-mode listing returned %d rows, want 2 (cross-repo): %+v", len(got), got)
	}
	repos := map[string]bool{}
	for _, r := range got {
		repos[r.RepoID] = true
	}
	if !repos["repo-A"] || !repos["repo-B"] {
		t.Errorf("expected rows tagged with both repo-A and repo-B; got %v", repos)
	}
}
