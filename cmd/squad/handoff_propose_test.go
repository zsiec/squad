package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/claims"
)

type proposeFixture struct {
	*chatFixture
	repoRoot string
	itemsDir string
}

func newProposeFixture(t *testing.T) *proposeFixture {
	t.Helper()
	cf := newChatFixture(t)
	repoRoot := t.TempDir()
	itemsDir := filepath.Join(repoRoot, ".squad", "items")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatalf("mkdir items: %v", err)
	}
	return &proposeFixture{chatFixture: cf, repoRoot: repoRoot, itemsDir: itemsDir}
}

func (p *proposeFixture) writeItemWithArea(t *testing.T, itemID, area string) {
	t.Helper()
	body := "---\nid: " + itemID + "\ntitle: dummy\ntype: task\npriority: P1\narea: " + area + "\nstatus: open\n---\n\n## Problem\n\nbody.\n"
	path := filepath.Join(p.itemsDir, itemID+"-stub.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
}

func (p *proposeFixture) postOnThread(t *testing.T, itemID, kind, body string) {
	t.Helper()
	if err := p.chat.Post(context.Background(), chat.PostRequest{
		AgentID: p.agentID,
		Thread:  itemID,
		Kind:    kind,
		Body:    body,
	}); err != nil {
		t.Fatalf("post: %v", err)
	}
}

func runPropose(t *testing.T, p *proposeFixture, h chat.HandoffBody, opts handoffOpts) *HandoffResult {
	t.Helper()
	st := claims.New(p.db, p.repoID, func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) })
	res, err := Handoff(context.Background(), HandoffArgs{
		Chat:                 p.chat,
		ClaimStore:           st,
		DB:                   p.db,
		RepoID:               p.repoID,
		RepoRoot:             p.repoRoot,
		ItemsDir:             p.itemsDir,
		AgentID:              p.agentID,
		Shipped:              h.Shipped,
		InFlight:             h.InFlight,
		SurprisedBy:          h.SurprisedBy,
		Unblocks:             h.Unblocks,
		Note:                 h.Note,
		ProposeFromSurprises: opts.ProposeFromSurprises,
		DryRun:               opts.DryRun,
		MaxProposals:         opts.MaxProposals,
	})
	if err != nil {
		t.Fatalf("Handoff: %v", err)
	}
	return res
}

func TestHandoffPropose_ExplicitSurprises(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-100")
	p.writeItemWithArea(t, "BUG-100", "store")

	res := runPropose(t, p, chat.HandoffBody{
		SurprisedBy: []string{"sqlite WAL eats fsync on macos", "modernc requires utf8 hex blobs"},
	}, handoffOpts{ProposeFromSurprises: true})

	if len(res.Proposals) != 2 {
		t.Fatalf("want 2 proposals, got %d", len(res.Proposals))
	}
	for i, d := range res.Proposals {
		if d.Path == "" {
			t.Errorf("proposal %d: empty path", i)
		}
		data, err := os.ReadFile(d.Path)
		if err != nil {
			t.Errorf("proposal %d: read stub: %v", i, err)
			continue
		}
		body := string(data)
		if !strings.Contains(body, "area: store") {
			t.Errorf("proposal %d: want area=store; body=%s", i, body)
		}
		if !strings.Contains(body, "kind: gotcha") {
			t.Errorf("proposal %d: want kind=gotcha; body=%s", i, body)
		}
	}
	if !strings.Contains(string(mustReadFile(t, res.Proposals[0].Path)), "sqlite WAL") {
		t.Error("proposal 0: Looks section should embed surprise body verbatim")
	}
}

func TestHandoffPropose_MinedFromHistory(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-200")
	p.writeItemWithArea(t, "BUG-200", "claims")

	p.postOnThread(t, "BUG-200", chat.KindStuck, "the lock didn't fire because the row was already there")
	p.postOnThread(t, "BUG-200", chat.KindFYI, "turns out modernc returns nil for empty BLOBs not an error")
	p.postOnThread(t, "BUG-200", chat.KindFYI, "didn't expect SELECT max() to scan when index covers")

	res := runPropose(t, p, chat.HandoffBody{Note: "wrap"}, handoffOpts{
		ProposeFromSurprises: true,
	})

	if len(res.Proposals) != 3 {
		t.Fatalf("want 3 mined proposals, got %d: %+v", len(res.Proposals), res.Proposals)
	}
	for _, d := range res.Proposals {
		if d.Area != "claims" {
			t.Errorf("proposal area=%s want claims", d.Area)
		}
	}
}

func TestHandoffPropose_MinedFromHistoryDeduplicates(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-201")
	p.writeItemWithArea(t, "BUG-201", "store")

	p.postOnThread(t, "BUG-201", chat.KindStuck, "lock contention on the same row")
	p.postOnThread(t, "BUG-201", chat.KindFYI, "turns out lock contention on the same row is the cause")

	res := runPropose(t, p, chat.HandoffBody{Note: "wrap"}, handoffOpts{ProposeFromSurprises: true})
	if len(res.Proposals) != 1 {
		t.Fatalf("want 1 deduped proposal, got %d", len(res.Proposals))
	}
}

func TestHandoffPropose_DryRunWritesNothing(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-300")
	p.writeItemWithArea(t, "BUG-300", "cli")

	res := runPropose(t, p, chat.HandoffBody{
		SurprisedBy: []string{"flag parsing eats trailing newline"},
	}, handoffOpts{ProposeFromSurprises: true, DryRun: true})

	if len(res.Proposals) != 1 {
		t.Fatalf("want 1 dry-run proposal, got %d", len(res.Proposals))
	}
	if !res.Proposals[0].DryRun {
		t.Error("expected DryRun=true on returned draft")
	}
	if _, err := os.Stat(res.Proposals[0].Path); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote file at %s; err=%v", res.Proposals[0].Path, err)
	}
}

func TestHandoffPropose_MaxCapClips(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-400")
	p.writeItemWithArea(t, "BUG-400", "cli")

	surprises := []string{
		"surprise alpha discovered when running tests",
		"surprise bravo discovered when running tests",
		"surprise charlie discovered when running tests",
		"surprise delta discovered when running tests",
		"surprise echo discovered when running tests",
		"surprise foxtrot discovered when running tests",
		"surprise golf discovered when running tests",
	}
	res := runPropose(t, p, chat.HandoffBody{SurprisedBy: surprises}, handoffOpts{
		ProposeFromSurprises: true,
		MaxProposals:         5,
	})
	if len(res.Proposals) != 5 {
		t.Fatalf("want 5 (capped), got %d", len(res.Proposals))
	}
	if !res.ProposalsClipped {
		t.Error("expected ProposalsClipped=true when 7 candidates capped at 5")
	}
}

func TestHandoffPropose_ZeroCandidates(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-500")
	p.writeItemWithArea(t, "BUG-500", "cli")

	p.postOnThread(t, "BUG-500", chat.KindThinking, "noise")
	p.postOnThread(t, "BUG-500", chat.KindMilestone, "shipped a thing")

	res := runPropose(t, p, chat.HandoffBody{Note: "nothing surprising"}, handoffOpts{
		ProposeFromSurprises: true,
	})

	if len(res.Proposals) != 0 {
		t.Errorf("want 0 proposals, got %d: %+v", len(res.Proposals), res.Proposals)
	}
	if res.ProposalsClipped {
		t.Error("clipped should be false on zero candidates")
	}
}

func TestHandoffPropose_DedupSurprisesCases(t *testing.T) {
	in := []surpriseCandidate{
		{Body: "lock contention on the row", Area: "store"},
		{Body: "lock contention on the row matters here", Area: "store"},
		{Body: "different finding entirely", Area: "store"},
	}
	out := dedupSurprises(in)
	if len(out) != 2 {
		t.Fatalf("want 2, got %d: %+v", len(out), out)
	}
}

// Short bodies must not subsume longer ones via substring. A KindStuck
// post like "wait" should not eat every later "wait, turns out X" candidate.
func TestHandoffPropose_DedupShortBodyDoesNotSubsume(t *testing.T) {
	in := []surpriseCandidate{
		{Body: "wait", Area: "cli"},
		{Body: "wait, the migration ran twice on a hot retry", Area: "cli"},
		{Body: "wait", Area: "cli"},
	}
	out := dedupSurprises(in)
	if len(out) != 2 {
		t.Fatalf("want 2 (short body keeps + longer body keeps; second short is exact-match dup), got %d: %+v", len(out), out)
	}
}

// Dry-run preview must walk the same collision suffixes the real path
// would, so an agent's preview matches the path that would be written
// when the same slug already exists on disk.
func TestHandoffPropose_DryRunWalksCollisions(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-700")
	p.writeItemWithArea(t, "BUG-700", "cli")

	// Land an existing proposal so the next derived slug collides.
	first := runPropose(t, p, chat.HandoffBody{
		SurprisedBy: []string{"flag parsing eats trailing newline"},
	}, handoffOpts{ProposeFromSurprises: true})
	if len(first.Proposals) != 1 {
		t.Fatalf("seed: want 1 proposal, got %d", len(first.Proposals))
	}
	baseSlug := first.Proposals[0].Slug

	// Dry-run with the same surprise — preview must show -2 suffix.
	preview := runPropose(t, p, chat.HandoffBody{
		SurprisedBy: []string{"flag parsing eats trailing newline"},
	}, handoffOpts{ProposeFromSurprises: true, DryRun: true})
	if len(preview.Proposals) != 1 {
		t.Fatalf("dry-run: want 1, got %d", len(preview.Proposals))
	}
	want := baseSlug + "-2"
	if preview.Proposals[0].Slug != want {
		t.Errorf("dry-run slug=%q want %q (collision walk)", preview.Proposals[0].Slug, want)
	}
}

func TestHandoffPropose_MatchesSurpriseKeywords(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"surprise: nothing here", true},
		{"I was surprised by the result", true},
		{"didn't expect the lock", true},
		{"turns out it was the index", true},
		{"wait, that's the bug", true},
		{"plain status update", false},
		{"shipped feat-9", false},
	}
	for _, c := range cases {
		if got := matchesSurprise(c.body); got != c.want {
			t.Errorf("matchesSurprise(%q)=%v want %v", c.body, got, c.want)
		}
	}
}

// TestHandoffPropose_Integration walks the cobra-layer runHandoffBody with
// --propose-from-surprises across a held claim with two stuck posts. Asserts
// both stubs land under .squad/learnings/gotchas/proposed/ with area=cli
// from the claim's item frontmatter, and that the handoff message itself
// posted normally and released the claim.
func TestHandoffPropose_Integration(t *testing.T) {
	p := newProposeFixture(t)
	p.insertClaim(t, "BUG-9999")
	p.writeItemWithArea(t, "BUG-9999", "cli")

	p.postOnThread(t, "BUG-9999", chat.KindStuck, "expected schemaJSON to canonicalise key order on round-trip")
	p.postOnThread(t, "BUG-9999", chat.KindStuck, "didn't expect cobra to swallow stderr from RunE on os.Exit")

	st := claims.New(p.db, p.repoID, func() time.Time { return time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC) })
	code := runHandoffBody(
		context.Background(), p.chat, st, p.db, p.repoID, p.repoRoot, p.agentID,
		chat.HandoffBody{Note: "wrap"},
		handoffOpts{ProposeFromSurprises: true, ItemsDir: p.itemsDir},
	)
	if code != 0 {
		t.Fatalf("handoff exit=%d", code)
	}

	proposedDir := filepath.Join(p.repoRoot, ".squad", "learnings", "gotchas", "proposed")
	entries, err := os.ReadDir(proposedDir)
	if err != nil {
		t.Fatalf("read proposed dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 stubs, got %d in %s", len(entries), proposedDir)
	}
	for _, e := range entries {
		body := mustReadFile(t, filepath.Join(proposedDir, e.Name()))
		if !strings.Contains(string(body), "area: cli") {
			t.Errorf("stub %s missing area=cli; body=%s", e.Name(), body)
		}
	}

	var open int
	_ = p.db.QueryRow(`SELECT COUNT(*) FROM claims WHERE agent_id = ?`, p.agentID).Scan(&open)
	if open != 0 {
		t.Errorf("claim still open after handoff: count=%d", open)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
