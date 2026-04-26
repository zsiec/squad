package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/repo"
)

const recaptureItemNeedsRefinement = `---
id: FEAT-501
title: Recapture me — refining agent finished editing
type: feature
priority: P1
area: auth
status: needs-refinement
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-A
captured_at: 1714150000
---

## Reviewer feedback
tighten the criteria

## Problem

Auth flow is racy.

## Acceptance criteria
- [ ] does the thing
`

func TestRecaptureCmd_HappyPath(t *testing.T) {
	repoDir := setupRefineRepo(t, "test-recapture-happy")
	writeItemFile(t, repoDir, "FEAT-501-x.md", recaptureItemNeedsRefinement)
	persistInboxFixture(t, repoDir, "FEAT-501-x.md")

	bc, err := bootClaimContext(context.Background())
	if err != nil {
		t.Fatalf("boot: %v", err)
	}
	insertClaimRow(t, bc, "FEAT-501")
	bc.Close()

	var stdout, stderr bytes.Buffer
	code := runRecapture(context.Background(), "FEAT-501", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "recaptured FEAT-501") {
		t.Fatalf("stdout missing 'recaptured FEAT-501': %s", stdout.String())
	}

	root, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	itemPath := filepath.Join(root, ".squad", "items", "FEAT-501-x.md")
	raw, err := os.ReadFile(itemPath)
	if err != nil {
		t.Fatalf("read item: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "status: captured") {
		t.Fatalf("frontmatter status not flipped to captured:\n%s", body)
	}
	if strings.Contains(body, "## Reviewer feedback") {
		t.Fatalf("reviewer feedback should be gone:\n%s", body)
	}
	if !strings.Contains(body, "## Refinement history") {
		t.Fatalf("refinement history section missing:\n%s", body)
	}
}

func TestRecaptureCmd_RequiresClaim(t *testing.T) {
	repoDir := setupRefineRepo(t, "test-recapture-no-claim")
	writeItemFile(t, repoDir, "FEAT-501-x.md", recaptureItemNeedsRefinement)
	persistInboxFixture(t, repoDir, "FEAT-501-x.md")

	var stdout, stderr bytes.Buffer
	code := runRecapture(context.Background(), "FEAT-501", &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit when no claim held\nstdout=%s\nstderr=%s",
			stdout.String(), stderr.String())
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "claim") {
		t.Fatalf("stderr should mention claim, got: %s", stderr.String())
	}
}

func insertClaimRow(t *testing.T, bc *claimContext, itemID string) {
	t.Helper()
	if _, err := bc.db.ExecContext(context.Background(),
		`INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		 VALUES (?, ?, ?, 0, 0, '', 0)`,
		bc.repoID, itemID, bc.agentID,
	); err != nil {
		t.Fatalf("insert claim: %v", err)
	}
}
