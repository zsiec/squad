package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func setupIntakeE2ERepo(t *testing.T, sessionID string) string {
	t.Helper()
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", sessionID)
	t.Setenv("SQUAD_AGENT", "agent-test")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.Remove(filepath.Join(repoDir, ".squad", "items", "EXAMPLE-001-try-the-loop.md")); err != nil {
		t.Fatalf("remove example item: %v", err)
	}
	return repoDir
}

func findIntakeItemPath(t *testing.T, repoDir, idGlob string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repoDir, ".squad", "items", idGlob+"-*.md"))
	if err != nil {
		t.Fatalf("glob %s: %v", idGlob, err)
	}
	if len(matches) != 1 {
		t.Fatalf("want exactly one match for %s, got %d: %v", idGlob, len(matches), matches)
	}
	return matches[0]
}

func TestIntake_E2E_FullFlow(t *testing.T) {
	repoDir := setupIntakeE2ERepo(t, "test-intake-e2e-full")
	ctx := context.Background()

	var newOut bytes.Buffer
	if code := runNew([]string{"feat", "thing"}, &newOut, items.Options{}); code != 0 {
		t.Fatalf("new: exit=%d stdout=%q", code, newOut.String())
	}
	if !strings.Contains(newOut.String(), "captured FEAT-001") {
		t.Fatalf("new stdout missing 'captured FEAT-001': %q", newOut.String())
	}

	var inboxOut, inboxErr bytes.Buffer
	if code := runInbox(ctx, inboxOpts{}, &inboxOut, &inboxErr); code != 0 {
		t.Fatalf("inbox: exit=%d stdout=%q stderr=%q", code, inboxOut.String(), inboxErr.String())
	}
	if !strings.Contains(inboxOut.String(), "FEAT-001") {
		t.Fatalf("inbox stdout missing FEAT-001: %q", inboxOut.String())
	}

	var acceptOut, acceptErr bytes.Buffer
	if code := runAccept(ctx, []string{"FEAT-001"}, &acceptOut, &acceptErr); code != 1 {
		t.Fatalf("first accept: exit=%d want 1 (DoR fail)\nstdout=%q\nstderr=%q",
			code, acceptOut.String(), acceptErr.String())
	}
	if !strings.Contains(acceptErr.String(), "area") {
		t.Fatalf("first accept stderr missing 'area' violation: %q", acceptErr.String())
	}

	itemPath := findIntakeItemPath(t, repoDir, "FEAT-001")
	body, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatalf("read item file: %v", err)
	}
	if !bytes.Contains(body, []byte("area: <fill-in>")) {
		t.Fatalf("item file missing 'area: <fill-in>' before edit:\n%s", body)
	}
	if !bytes.Contains(body, []byte("- [ ] Specific, testable thing 1")) {
		t.Fatalf("item file missing default AC checkbox:\n%s", body)
	}
	body = bytes.Replace(body, []byte("area: <fill-in>"), []byte("area: auth"), 1)
	body = bytes.Replace(body,
		[]byte("- [ ] Specific, testable thing 1\n- [ ] Specific, testable thing 2\n"),
		[]byte("- [ ] the rule replaces the placeholder body verbatim\n"), 1)
	if err := os.WriteFile(itemPath, body, 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}

	var acceptOut2, acceptErr2 bytes.Buffer
	if code := runAccept(ctx, []string{"FEAT-001"}, &acceptOut2, &acceptErr2); code != 0 {
		t.Fatalf("second accept: exit=%d want 0\nstdout=%q\nstderr=%q",
			code, acceptOut2.String(), acceptErr2.String())
	}
	if !strings.Contains(acceptOut2.String(), "accepted FEAT-001") {
		t.Fatalf("second accept stdout missing 'accepted FEAT-001': %q", acceptOut2.String())
	}

	parsed, err := items.Parse(itemPath)
	if err != nil {
		t.Fatalf("parse after accept: %v", err)
	}
	if parsed.Status != "open" {
		t.Errorf("status=%q want open", parsed.Status)
	}
	if parsed.AcceptedBy != "agent-test" {
		t.Errorf("accepted_by=%q want agent-test", parsed.AcceptedBy)
	}
	if parsed.AcceptedAt == 0 {
		t.Errorf("accepted_at=0 want non-zero")
	}

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		t.Fatalf("repo id: %v", err)
	}
	var dbStatus, dbAcceptedBy string
	if err := db.QueryRow(
		`SELECT status, COALESCE(accepted_by,'') FROM items WHERE repo_id=? AND item_id=?`,
		repoID, "FEAT-001",
	).Scan(&dbStatus, &dbAcceptedBy); err != nil {
		t.Fatalf("query items row: %v", err)
	}
	if dbStatus != "open" {
		t.Errorf("db status=%q want open", dbStatus)
	}
	if dbAcceptedBy != "agent-test" {
		t.Errorf("db accepted_by=%q want agent-test", dbAcceptedBy)
	}

	var nextOut bytes.Buffer
	if code := runNext(nil, &nextOut, false, 0, false); code != 0 {
		t.Fatalf("next: exit=%d stdout=%q", code, nextOut.String())
	}
	if !strings.Contains(nextOut.String(), "FEAT-001") {
		t.Fatalf("next stdout missing FEAT-001: %q", nextOut.String())
	}

	var newOut2 bytes.Buffer
	if code := runNew([]string{"feat", "to be rejected"}, &newOut2, items.Options{}); code != 0 {
		t.Fatalf("second new: exit=%d stdout=%q", code, newOut2.String())
	}
	if !strings.Contains(newOut2.String(), "captured FEAT-002") {
		t.Fatalf("second new stdout missing 'captured FEAT-002': %q", newOut2.String())
	}

	var rejectOut, rejectErr bytes.Buffer
	if code := runReject(ctx, []string{"FEAT-002"}, "duplicate", &rejectOut, &rejectErr); code != 0 {
		t.Fatalf("reject: exit=%d stdout=%q stderr=%q",
			code, rejectOut.String(), rejectErr.String())
	}
	matches, err := filepath.Glob(filepath.Join(repoDir, ".squad", "items", "FEAT-002-*.md"))
	if err != nil {
		t.Fatalf("glob FEAT-002: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("FEAT-002 file not removed after reject: %v", matches)
	}
	logBody, err := os.ReadFile(filepath.Join(repoDir, ".squad", "rejected.log"))
	if err != nil {
		t.Fatalf("read rejected.log: %v", err)
	}
	for _, want := range []string{"FEAT-002", "duplicate"} {
		if !strings.Contains(string(logBody), want) {
			t.Errorf("rejected.log missing %q:\n%s", want, logBody)
		}
	}
}

func TestIntake_E2E_ReadyFlagBypassesInbox(t *testing.T) {
	setupIntakeE2ERepo(t, "test-intake-e2e-ready")
	ctx := context.Background()

	var newOut bytes.Buffer
	if code := runNew([]string{"feat", "x"}, &newOut, items.Options{Ready: true}); code != 0 {
		t.Fatalf("new --ready: exit=%d stdout=%q", code, newOut.String())
	}
	if !strings.Contains(newOut.String(), "ready FEAT-001") {
		t.Fatalf("new --ready stdout missing 'ready FEAT-001': %q", newOut.String())
	}

	var inboxOut, inboxErr bytes.Buffer
	if code := runInbox(ctx, inboxOpts{}, &inboxOut, &inboxErr); code != 0 {
		t.Fatalf("inbox: exit=%d stdout=%q stderr=%q", code, inboxOut.String(), inboxErr.String())
	}
	if !strings.Contains(inboxOut.String(), "inbox: empty") {
		t.Fatalf("inbox should be empty after --ready: %q", inboxOut.String())
	}
	if strings.Contains(inboxOut.String(), "FEAT-001") {
		t.Fatalf("inbox should not include the --ready item: %q", inboxOut.String())
	}

	var nextOut bytes.Buffer
	if code := runNext(nil, &nextOut, false, 0, false); code != 0 {
		t.Fatalf("next: exit=%d stdout=%q", code, nextOut.String())
	}
	if !strings.Contains(nextOut.String(), "FEAT-001") {
		t.Fatalf("next stdout missing FEAT-001: %q", nextOut.String())
	}
}
