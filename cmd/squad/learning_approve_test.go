package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearningApprove_MovesAndRewritesState(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mkLearning(t, repo, "gotchas/proposed/sqlite-busy.md", "gotcha", "sqlite-busy", "store", "proposed")

	root := newRootCmd()
	root.SetArgs([]string{"learning", "approve", "sqlite-busy"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	old := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed", "sqlite-busy.md")
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("expected old path gone, got %v", err)
	}
	got := filepath.Join(repo, ".squad", "learnings", "gotchas", "approved", "sqlite-busy.md")
	body, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read %s: %v", got, err)
	}
	if !strings.Contains(string(body), "state: approved") || strings.Contains(string(body), "state: proposed") {
		t.Errorf("state line not rewritten:\n%s", body)
	}
}

func TestLearningApprove_AmbiguousSlugErrors(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	mkLearning(t, repo, "gotchas/proposed/x.md", "gotcha", "x", "a", "proposed")
	mkLearning(t, repo, "patterns/proposed/x.md", "pattern", "x", "a", "proposed")

	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "approve", "x"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error()+stderr.String(), "ambiguous") {
		t.Fatalf("want ambiguity error, got %v / %s", err, stderr.String())
	}
}

func TestLearningApprove_RefusesMalformed(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)
	dir := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nid: gotcha-bad\nkind: gotcha\nslug: bad\ntitle: t\narea: a\ncreated: 2026-04-25\ncreated_by: x\nsession: s\nstate: proposed\n---\n\n## Looks like\n\nincomplete\n"
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRootCmd()
	root.SetArgs([]string{"learning", "approve", "bad"})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected approve to fail on malformed")
	}
}
