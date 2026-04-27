package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/intake"
	"github.com/zsiec/squad/internal/store"
)

// setupIntakeRepo lays out a fake repo + .squad/, points SQUAD_HOME at a
// fresh tmp dir so OpenDefault() lands on a private DB, and chdirs in so
// repo.Discover finds the right root. Returns the repo root (== wd at exit).
func setupIntakeRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".squad", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := []byte("id_prefixes: [BUG, FEAT, TASK, CHORE]\n")
	if err := os.WriteFile(filepath.Join(repoRoot, ".squad", "config.yaml"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("SQUAD_HOME", home)
	if err := store.EnsureHome(); err != nil {
		t.Fatalf("ensure home: %v", err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	return repoRoot
}

func TestIntakeNew_PrintsSessionIDAndBriefing(t *testing.T) {
	setupIntakeRepo(t)
	var stdout, stderr bytes.Buffer
	code := runIntakeNew(context.Background(),
		[]string{"rotate", "keys", "without", "downtime"},
		&stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "opened session intake-") {
		t.Errorf("missing session line; got %q", out)
	}
	if !strings.Contains(out, "intake-checklist.yaml") {
		t.Errorf("missing skill briefing; got %q", out)
	}
}

func TestIntakeNew_ResumesExisting(t *testing.T) {
	setupIntakeRepo(t)
	var first bytes.Buffer
	if code := runIntakeNew(context.Background(), []string{"first idea"}, &first, os.Stderr); code != 0 {
		t.Fatalf("first: %d", code)
	}
	var second bytes.Buffer
	if code := runIntakeNew(context.Background(), []string{"second idea"}, &second, os.Stderr); code != 0 {
		t.Fatalf("second: %d", code)
	}
	if !strings.Contains(second.String(), "resumed existing session") {
		t.Errorf("second open should resume; got %q", second.String())
	}
}

func TestIntakeList_EmptyAndAfterOpen(t *testing.T) {
	setupIntakeRepo(t)
	var empty bytes.Buffer
	if code := runIntakeList(context.Background(), &empty, os.Stderr); code != 0 {
		t.Fatalf("empty list: %d", code)
	}
	if !strings.Contains(empty.String(), "no open intake sessions") {
		t.Errorf("empty list output: %q", empty.String())
	}

	if code := runIntakeNew(context.Background(), []string{"rotate keys"}, &bytes.Buffer{}, os.Stderr); code != 0 {
		t.Fatalf("open: %d", code)
	}
	var after bytes.Buffer
	if code := runIntakeList(context.Background(), &after, os.Stderr); code != 0 {
		t.Fatalf("post-open list: %d", code)
	}
	if !strings.Contains(after.String(), "intake-") {
		t.Errorf("post-open list missing session id: %q", after.String())
	}
	if !strings.Contains(after.String(), "rotate keys") {
		t.Errorf("post-open list missing idea: %q", after.String())
	}
}

func TestIntakeStatus_PrintsTranscriptAndStillRequired(t *testing.T) {
	repoRoot := setupIntakeRepo(t)

	var openOut bytes.Buffer
	if code := runIntakeNew(context.Background(), []string{"rotate keys"}, &openOut, os.Stderr); code != 0 {
		t.Fatalf("open: %d", code)
	}
	sessID := extractSessionID(t, openOut.String())

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	checklist, err := intake.LoadChecklist(filepath.Join(repoRoot, ".squad"))
	if err != nil {
		t.Fatalf("checklist: %v", err)
	}
	if _, _, err := intake.AppendTurn(context.Background(), db, checklist,
		sessID, agentIDOrFatal(t), "user", "rotate keys without downtime",
		[]string{"intent"}); err != nil {
		t.Fatalf("append turn: %v", err)
	}

	var statusOut bytes.Buffer
	if code := runIntakeStatus(context.Background(), sessID, &statusOut, os.Stderr); code != 0 {
		t.Fatalf("status: %d stderr=ok", code)
	}
	out := statusOut.String()
	if !strings.Contains(out, sessID) {
		t.Errorf("status missing id: %q", out)
	}
	if !strings.Contains(out, "rotate keys without downtime") {
		t.Errorf("status missing transcript content: %q", out)
	}
	if !strings.Contains(out, "still required") {
		t.Errorf("status missing still_required: %q", out)
	}
}

func TestIntakeRefine_OpensSessionAndPrintsSnapshot(t *testing.T) {
	repoRoot := setupIntakeRepo(t)
	itemDir := filepath.Join(repoRoot, ".squad", "items")
	frontmatter := "---\n" +
		"id: FEAT-100\n" +
		"title: rotate keys\n" +
		"type: feature\n" +
		"priority: P2\n" +
		"area: auth\n" +
		"status: captured\n" +
		"estimate: 1h\n" +
		"risk: low\n" +
		"created: 2026-04-26\n" +
		"updated: 2026-04-26\n" +
		"captured_by: agent-1\n" +
		"captured_at: 1714150000\n" +
		"---\n\n## Problem\nold problem\n"
	itemPath := filepath.Join(itemDir, "FEAT-100-rotate-keys.md")
	if err := os.WriteFile(itemPath, []byte(frontmatter), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := runIntakeRefine(context.Background(), "FEAT-100", &stdout, &stderr); code != 0 {
		t.Fatalf("refine: %d stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "opened refine session intake-") {
		t.Errorf("missing refine session line: %q", out)
	}
	if !strings.Contains(out, "FEAT-100") {
		t.Errorf("missing item id in output: %q", out)
	}
	if !strings.Contains(out, "rotate keys") {
		t.Errorf("missing snapshot title: %q", out)
	}
	if !strings.Contains(out, "auth") {
		t.Errorf("missing snapshot area: %q", out)
	}
}

func TestIntakeCancel_MarksClosed(t *testing.T) {
	setupIntakeRepo(t)
	var openOut bytes.Buffer
	if code := runIntakeNew(context.Background(), []string{"x"}, &openOut, os.Stderr); code != 0 {
		t.Fatalf("open: %d", code)
	}
	sessID := extractSessionID(t, openOut.String())

	var cancelOut bytes.Buffer
	if code := runIntakeCancel(context.Background(), sessID, &cancelOut, os.Stderr); code != 0 {
		t.Fatalf("cancel: %d", code)
	}
	if !strings.Contains(cancelOut.String(), "cancelled") {
		t.Errorf("cancel output: %q", cancelOut.String())
	}

	var listOut bytes.Buffer
	_ = runIntakeList(context.Background(), &listOut, os.Stderr)
	if !strings.Contains(listOut.String(), "no open intake sessions") {
		t.Errorf("post-cancel list should be empty: %q", listOut.String())
	}
}

func TestIntakeCommit_E2E_ItemOnly(t *testing.T) {
	repoRoot := setupIntakeRepo(t)

	var openOut bytes.Buffer
	if code := runIntakeNew(context.Background(), []string{"rotate keys"}, &openOut, os.Stderr); code != 0 {
		t.Fatalf("open: %d", code)
	}
	sessID := extractSessionID(t, openOut.String())

	bundle := intake.Bundle{Items: []intake.ItemDraft{{
		Title: "rotate keys cleanly", Intent: "online rotation",
		Acceptance: []string{"keys rotate", "no downtime"}, Area: "auth",
	}}}
	body, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(bundlePath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	var commitOut bytes.Buffer
	if code := runIntakeCommit(context.Background(), sessID, bundlePath, false, &commitOut, os.Stderr); code != 0 {
		t.Fatalf("commit: %d", code)
	}
	out := commitOut.String()
	if !strings.Contains(out, "committed "+sessID) {
		t.Errorf("commit missing summary: %q", out)
	}
	if !strings.Contains(out, "shape=item_only") {
		t.Errorf("commit shape line: %q", out)
	}

	matches, err := filepath.Glob(filepath.Join(repoRoot, ".squad", "items", "FEAT-*.md"))
	if err != nil || len(matches) == 0 {
		t.Errorf("no item file written: matches=%v err=%v", matches, err)
	}
}

func extractSessionID(t *testing.T, line string) string {
	t.Helper()
	for _, w := range strings.Fields(line) {
		if strings.HasPrefix(w, "intake-") {
			return strings.TrimSpace(w)
		}
	}
	t.Fatalf("no session id found in: %q", line)
	return ""
}

func agentIDOrFatal(t *testing.T) string {
	t.Helper()
	// Open a quick context to read the agent id off the same path the
	// run* functions use.
	var stderr bytes.Buffer
	ic, code := newIntakeContext(&stderr)
	if code != 0 {
		t.Fatalf("newIntakeContext: %s", stderr.String())
	}
	defer ic.close()
	return ic.agentID
}
