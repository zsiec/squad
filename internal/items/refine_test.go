package items

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
	_ "modernc.org/sqlite"
)

func TestWriteFeedback_InsertsAboveProblem(t *testing.T) {
	in := "## Problem\nfoo\n\n## Acceptance criteria\n- [ ] x\n"
	got := WriteFeedback(in, "rename area")
	want := "## Reviewer feedback\nrename area\n\n## Problem\nfoo\n\n## Acceptance criteria\n- [ ] x\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestWriteFeedback_ReplacesExisting(t *testing.T) {
	in := "## Reviewer feedback\nold note\n\n## Problem\nfoo\n"
	got := WriteFeedback(in, "new note")
	want := "## Reviewer feedback\nnew note\n\n## Problem\nfoo\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestWriteFeedback_NoProblemSection_PrependsAtTop(t *testing.T) {
	in := "freeform body without sections\n"
	got := WriteFeedback(in, "needs structure")
	want := "## Reviewer feedback\nneeds structure\n\nfreeform body without sections\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestWriteFeedback_TrailingNewlinePreserved(t *testing.T) {
	in := "## Problem\nfoo\n"
	got := WriteFeedback(in, "fix")
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

func TestMoveFeedbackToHistory_FirstRound(t *testing.T) {
	in := "## Reviewer feedback\nplease tighten\n\n## Problem\nfoo\n"
	got := MoveFeedbackToHistory(in, "2026-04-26")
	want := "## Refinement history\n### Round 1 — 2026-04-26\nplease tighten\n\n## Problem\nfoo\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestMoveFeedbackToHistory_AppendsRound(t *testing.T) {
	in := "## Reviewer feedback\nround 2 ask\n\n## Refinement history\n### Round 1 — 2026-04-25\nfirst ask\n\n## Problem\nfoo\n"
	got := MoveFeedbackToHistory(in, "2026-04-26")
	want := "## Refinement history\n### Round 1 — 2026-04-25\nfirst ask\n\n### Round 2 — 2026-04-26\nround 2 ask\n\n## Problem\nfoo\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestMoveFeedbackToHistory_NoFeedback_Noop(t *testing.T) {
	in := "## Problem\nfoo\n"
	got := MoveFeedbackToHistory(in, "2026-04-26")
	if got != in {
		t.Fatalf("expected no-op, got: %q", got)
	}
}

func TestRewriteWithFeedback_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "FEAT-001-x.md")
	body := "---\nid: FEAT-001\ntitle: x\nstatus: captured\nupdated: 2026-04-25\n---\n\n## Problem\nfoo\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	if err := RewriteWithFeedback(path, "rename area", "needs-refinement", now); err != nil {
		t.Fatalf("RewriteWithFeedback: %v", err)
	}
	it, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse after rewrite: %v", err)
	}
	if it.Status != "needs-refinement" {
		t.Fatalf("status not flipped: %q", it.Status)
	}
	if it.Updated != "2026-04-26" {
		t.Fatalf("updated not advanced: %q", it.Updated)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "## Reviewer feedback\nrename area\n") {
		t.Fatalf("feedback section missing:\n%s", got)
	}
	if !strings.Contains(string(got), "## Problem\nfoo\n") {
		t.Fatalf("original body lost:\n%s", got)
	}
}

func TestRewriteRecapture_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "FEAT-002-x.md")
	body := "---\nid: FEAT-002\ntitle: x\nstatus: needs-refinement\nupdated: 2026-04-25\n---\n\n## Reviewer feedback\nplease tighten\n\n## Problem\nfoo\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	if err := RewriteRecapture(path, "2026-04-26", "captured", now); err != nil {
		t.Fatalf("RewriteRecapture: %v", err)
	}
	it, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse after rewrite: %v", err)
	}
	if it.Status != "captured" {
		t.Fatalf("status not flipped: %q", it.Status)
	}
	if it.Updated != "2026-04-26" {
		t.Fatalf("updated not advanced: %q", it.Updated)
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "## Reviewer feedback") {
		t.Fatalf("reviewer feedback section should be gone:\n%s", got)
	}
	if !strings.Contains(string(got), "## Refinement history") {
		t.Fatalf("refinement history section missing:\n%s", got)
	}
	if !strings.Contains(string(got), "### Round 1 — 2026-04-26") {
		t.Fatalf("round 1 entry missing:\n%s", got)
	}
	if !strings.Contains(string(got), "## Problem\nfoo\n") {
		t.Fatalf("original body lost:\n%s", got)
	}
}

func TestRewriteRecapture_AcceptsBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "FEAT-002-x.md")
	body := "\xef\xbb\xbf---\nid: FEAT-002\ntitle: x\nstatus: needs-refinement\nupdated: 2026-04-25\n---\n\n## Reviewer feedback\nfix it\n\n## Problem\nfoo\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	if err := RewriteRecapture(path, "2026-04-26", "captured", now); err != nil {
		t.Fatalf("RewriteRecapture on BOM file failed: %v", err)
	}
	it, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse after rewrite: %v", err)
	}
	if it.Status != "captured" {
		t.Fatalf("status not flipped on BOM file: %q", it.Status)
	}
	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "## Reviewer feedback") {
		t.Fatalf("reviewer feedback section should be gone on BOM file:\n%s", got)
	}
	if !strings.Contains(string(got), "### Round 1 — 2026-04-26") {
		t.Fatalf("round 1 entry missing on BOM file:\n%s", got)
	}
}

func TestRefine_FlipsStatusAndPersists(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	seeded := seedPromotableItem(t, ctx, db, squadDir, "r", "FEAT-700")

	if err := Refine(ctx, db, "r", seeded.ID, "tighten the criteria"); err != nil {
		t.Fatalf("refine: %v", err)
	}

	var dbStatus string
	if err := db.QueryRow(
		`SELECT status FROM items WHERE item_id=?`, seeded.ID,
	).Scan(&dbStatus); err != nil {
		t.Fatalf("db scan: %v", err)
	}
	if dbStatus != "needs-refinement" {
		t.Fatalf("db status=%q want needs-refinement", dbStatus)
	}
	raw, err := os.ReadFile(seeded.Path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(raw), "## Reviewer feedback") {
		t.Fatalf("body missing reviewer feedback section:\n%s", raw)
	}
	if !strings.Contains(string(raw), "tighten the criteria") {
		t.Fatalf("body missing comment text:\n%s", raw)
	}
}

func TestRefine_RejectsEmptyComments(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	seeded := seedPromotableItem(t, ctx, db, squadDir, "r", "FEAT-701")

	if err := Refine(ctx, db, "r", seeded.ID, ""); !errors.Is(err, ErrCommentsRequired) {
		t.Fatalf("want ErrCommentsRequired, got: %v", err)
	}
	if err := Refine(ctx, db, "r", seeded.ID, "   "); !errors.Is(err, ErrCommentsRequired) {
		t.Fatalf("whitespace-only comments should also fail: %v", err)
	}
}

func TestRefine_RejectsWrongStatus(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	p := filepath.Join(squadDir, "items", "FEAT-702-thing.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `---
id: FEAT-702
title: investigate the flaky auth test we have
type: feat
status: open
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] x
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	it, _ := Parse(p)
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	if err := Refine(ctx, db, "r", "FEAT-702", "please redo"); !errors.Is(err, ErrWrongStatusForRefine) {
		t.Fatalf("want ErrWrongStatusForRefine, got: %v", err)
	}
}

func TestRefine_AllowsNeedsRefinementStatus(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	squadDir := filepath.Join(dir, ".squad")
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	p := filepath.Join(squadDir, "items", "FEAT-703-thing.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := `---
id: FEAT-703
title: investigate the flaky auth test we have
type: feat
status: needs-refinement
priority: P2
area: auth
estimate: 1h
risk: low
created: 2026-04-26
updated: 2026-04-26
---

## Reviewer feedback
prior round

## Acceptance criteria
- [ ] x
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	it, _ := Parse(p)
	if err := Persist(ctx, db, "r", it, false); err != nil {
		t.Fatalf("persist: %v", err)
	}
	if err := Refine(ctx, db, "r", "FEAT-703", "another round"); err != nil {
		t.Fatalf("refine: %v", err)
	}
	raw, _ := os.ReadFile(p)
	if !strings.Contains(string(raw), "another round") {
		t.Fatalf("body missing latest comments:\n%s", raw)
	}
}

func TestRefine_MissingItemReturnsErrItemNotFound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, _ := store.Open(filepath.Join(dir, "test.db"))
	defer db.Close()
	if err := Refine(ctx, db, "r", "FEAT-999", "x"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("want ErrItemNotFound, got: %v", err)
	}
}

func TestRewriteWithFeedback_AcceptsBOM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "FEAT-001-x.md")
	body := "\xef\xbb\xbf---\nid: FEAT-001\ntitle: x\nstatus: captured\nupdated: 2026-04-25\n---\n\n## Problem\nfoo\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	if err := RewriteWithFeedback(path, "fix it", "needs-refinement", now); err != nil {
		t.Fatalf("RewriteWithFeedback on BOM file failed: %v", err)
	}
	it, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse after rewrite: %v", err)
	}
	if it.Status != "needs-refinement" {
		t.Fatalf("status not flipped on BOM file: %q", it.Status)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "## Reviewer feedback\nfix it\n") {
		t.Fatalf("feedback section missing on BOM file:\n%s", got)
	}
}
