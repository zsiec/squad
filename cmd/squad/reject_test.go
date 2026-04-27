package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

const rejectItemA = `---
id: FEAT-101
title: Reject me please for valid reasons
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] the rule does the thing as specified
`

const rejectItemB = `---
id: FEAT-102
title: Also captured but will be claimed
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] the rule does the thing as specified
`

func setupRejectRepo(t *testing.T, sessionID string) string {
	t.Helper()
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", sessionID)
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return repoDir
}

func persistRejectFixture(t *testing.T, repoDir, fileName string) {
	t.Helper()
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("repo.Discover: %v", err)
	}
	path := filepath.Join(root, ".squad", "items", fileName)
	parsed, err := items.Parse(path)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	parsed.Path = path
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	repoID, err := repo.IDFor(root)
	if err != nil {
		t.Fatalf("repo.IDFor: %v", err)
	}
	if err := items.Persist(context.Background(), db, repoID, parsed, false); err != nil {
		t.Fatalf("persist %s: %v", fileName, err)
	}
}

func seedClaim(t *testing.T, repoDir, itemID, agentID string) {
	t.Helper()
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("repo.Discover: %v", err)
	}
	repoID, err := repo.IDFor(root)
	if err != nil {
		t.Fatalf("repo.IDFor: %v", err)
	}
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	now := time.Now().Unix()
	_, err = db.Exec(`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, repoID, itemID, agentID, now, now, "", 0)
	if err != nil {
		t.Fatalf("seed claim: %v", err)
	}
}

func itemRowExists(t *testing.T, itemID string) bool {
	t.Helper()
	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM items WHERE item_id=?`, itemID).Scan(&n); err != nil {
		t.Fatalf("query item count for %s: %v", itemID, err)
	}
	return n > 0
}

func TestRunReject_HappyPath(t *testing.T) {
	repoDir := setupRejectRepo(t, "test-reject-pass")
	writeItemFile(t, repoDir, "FEAT-101-reject.md", rejectItemA)
	persistRejectFixture(t, repoDir, "FEAT-101-reject.md")

	var stdout, stderr bytes.Buffer
	code := runReject(context.Background(), []string{"FEAT-101"}, "out of scope", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "rejected FEAT-101") {
		t.Fatalf("stdout missing 'rejected FEAT-101': %s", stdout.String())
	}
	if itemRowExists(t, "FEAT-101") {
		t.Fatalf("FEAT-101 row still exists after reject")
	}
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	itemPath := filepath.Join(root, ".squad", "items", "FEAT-101-reject.md")
	if _, err := items.Parse(itemPath); err == nil {
		t.Fatalf("FEAT-101 file still present after reject")
	}
	logBytes, err := os.ReadFile(filepath.Join(root, ".squad", "rejected.log"))
	if err != nil {
		t.Fatalf("read rejected.log: %v", err)
	}
	if !strings.Contains(string(logBytes), "FEAT-101") || !strings.Contains(string(logBytes), "out of scope") {
		t.Fatalf("rejected.log missing entry for FEAT-101 or reason: %s", string(logBytes))
	}
}

func TestRunReject_PartialSuccess(t *testing.T) {
	repoDir := setupRejectRepo(t, "test-reject-partial")
	writeItemFile(t, repoDir, "FEAT-101-reject.md", rejectItemA)
	writeItemFile(t, repoDir, "FEAT-102-claimed.md", rejectItemB)
	persistRejectFixture(t, repoDir, "FEAT-101-reject.md")
	persistRejectFixture(t, repoDir, "FEAT-102-claimed.md")
	seedClaim(t, repoDir, "FEAT-102", "another-agent")

	var stdout, stderr bytes.Buffer
	code := runReject(context.Background(), []string{"FEAT-101", "FEAT-102"}, "duplicate", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (partial)\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "rejected FEAT-101") {
		t.Fatalf("stdout missing 'rejected FEAT-101': %s", stdout.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "FEAT-102") {
		t.Fatalf("stderr missing FEAT-102: %s", errOut)
	}
	if !strings.Contains(errOut, "claimed") {
		t.Fatalf("stderr missing 'claimed' for FEAT-102: %s", errOut)
	}
	if itemRowExists(t, "FEAT-101") {
		t.Fatalf("FEAT-101 row still exists after reject")
	}
	if !itemRowExists(t, "FEAT-102") {
		t.Fatalf("FEAT-102 row missing — claimed item should not have been deleted")
	}
}

func TestRunReject_UnknownIDIsNoop(t *testing.T) {
	repoDir := setupRejectRepo(t, "test-reject-unknown")

	var stdout, stderr bytes.Buffer
	code := runReject(context.Background(), []string{"FEAT-999"}, "ghost", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0 (unknown id is no-op)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if _, err := os.ReadFile(filepath.Join(root, ".squad", "rejected.log")); err == nil {
		t.Fatalf("rejected.log was created for unknown id (expected no-op)")
	}
}

func TestRunReject_ReasonRequiredByCobra(t *testing.T) {
	setupRejectRepo(t, "test-reject-no-reason")

	cmd := newRejectCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"FEAT-101"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected cobra to reject missing --reason, got nil err\nstdout=%s\nstderr=%s",
			stdout.String(), stderr.String())
	}
	if !strings.Contains(strings.ToLower(err.Error()), "reason") {
		t.Fatalf("error should mention required reason flag, got: %v", err)
	}
}

func TestRunReject_NoIDsRejectedByCobra(t *testing.T) {
	setupRejectRepo(t, "test-reject-no-args")

	cmd := newRejectCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--reason", "x"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected cobra Args validator to reject empty arg list, got nil err\nstdout=%s\nstderr=%s",
			stdout.String(), stderr.String())
	}
}
