package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/attest"
)

const tierItemFmt = `---
id: %s
title: a sufficiently long title for ready
type: bug
priority: %s
area: core
status: open
estimate: 1h
risk: %s
created: 2026-04-25
updated: 2026-04-25
---

## Acceptance criteria
- [ ] x
`

func writeTierItem(t *testing.T, itemsDir, id, priority, risk string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(itemsDir, id+"-x.md"),
		[]byte(fmt.Sprintf(tierItemFmt, id, priority, risk)),
		0o644); err != nil {
		t.Fatal(err)
	}
}

// seedReviewRow seeds a passing review row for the given
// reviewer-agent on the item, mirroring what `squad attest --kind
// review --reviewer-agent <id> --findings-file <path>` writes.
func seedReviewRow(t *testing.T, env *testEnv, itemID, reviewerAgent string) {
	t.Helper()
	L := attest.New(env.DB, env.RepoID, nil)
	if _, err := L.Insert(context.Background(), attest.Record{
		ItemID:     itemID,
		Kind:       attest.KindReview,
		Command:    "review by " + reviewerAgent,
		ExitCode:   0,
		OutputHash: env.RepoID + "-" + itemID + "-" + reviewerAgent,
		AgentID:    env.AgentID,
	}); err != nil {
		t.Fatalf("seed review attestation %s: %v", reviewerAgent, err)
	}
}

func setupTierEnv(t *testing.T, id, priority, risk string) (*testEnv, string) {
	t.Helper()
	env := newTestEnv(t)
	writeTierItem(t, env.ItemsDir, id, priority, risk)
	gitFixtureCommit(t, env.Root)
	gitConfigUser(t, env.Root)
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: id, ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	return env, id
}

// TestDone_P0RequiresTwoDistinctReviewers covers AC#1: a P0 item
// cannot reach done until two reviewer attestations exist with
// distinct reviewer-agent values. With one review, Done errors.
func TestDone_P0RequiresTwoDistinctReviewers(t *testing.T) {
	env, id := setupTierEnv(t, "BUG-700", "P0", "low")
	seedReviewRow(t, env, id, "superpowers-code-reviewer")

	_, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: id, Summary: "ship",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	if err == nil {
		t.Fatal("expected error: P0 with single review must fail tier gate")
	}
	var miss *EvidenceMissingError
	if !errors.As(err, &miss) {
		t.Errorf("error type should be EvidenceMissingError; got %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "two distinct reviewers") &&
		!strings.Contains(err.Error(), "second reviewer") {
		t.Errorf("error should explain the missing second reviewer; got %v", err)
	}
}

// TestDone_P0PassesWithTwoDistinctReviewers covers AC#1 + AC#3: two
// distinct reviewer-agent attestations satisfy the gate. Done succeeds.
func TestDone_P0PassesWithTwoDistinctReviewers(t *testing.T) {
	env, id := setupTierEnv(t, "BUG-701", "P0", "low")
	seedReviewRow(t, env, id, "superpowers-code-reviewer")
	seedReviewRow(t, env, id, "superpowers-production-failure-reviewer")

	if _, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: id, Summary: "ship",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	}); err != nil {
		t.Fatalf("Done: %v", err)
	}
}

// TestDone_HighRiskRequiresTwoDistinctReviewers covers AC#2: risk:high
// fires the gate independent of priority. Tested with priority=P2.
func TestDone_HighRiskRequiresTwoDistinctReviewers(t *testing.T) {
	env, id := setupTierEnv(t, "BUG-702", "P2", "high")
	seedReviewRow(t, env, id, "superpowers-code-reviewer")

	_, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: id, Summary: "ship",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	if err == nil {
		t.Fatal("expected error: risk=high with single review must fail tier gate")
	}
}

// TestDone_TwoReviewsFromSameAgentDontCount enforces the AC's
// "distinct" requirement: two attestations from the same reviewer-agent
// is one reviewer, not two. The gate must fail.
func TestDone_TwoReviewsFromSameAgentDontCount(t *testing.T) {
	env, id := setupTierEnv(t, "BUG-703", "P0", "low")
	seedReviewRow(t, env, id, "superpowers-code-reviewer")
	seedReviewRow(t, env, id, "superpowers-code-reviewer")

	_, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: id, Summary: "ship",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	if err == nil {
		t.Fatal("two reviews from the same reviewer-agent should not satisfy the distinct-reviewers gate")
	}
}

// TestDone_LowerTierUnchanged covers AC#4: P2 + risk:low is the
// existing single-reviewer (or zero — evidence_required absent)
// behavior. No friction added.
func TestDone_LowerTierUnchanged(t *testing.T) {
	env, id := setupTierEnv(t, "BUG-704", "P2", "low")
	if _, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: id, Summary: "ship",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	}); err != nil {
		t.Fatalf("Done on P2 low-risk should pass with no review attestations: %v", err)
	}
}

// TestDone_HighRiskForceOverrideWorks: --force still bypasses the
// tier gate (records the bypass as a manual attestation per the
// existing override pattern). Operator-of-last-resort path.
func TestDone_HighRiskForceOverrideWorks(t *testing.T) {
	env, id := setupTierEnv(t, "BUG-705", "P2", "high")
	if _, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: id, Summary: "force ship",
		ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root, Force: true,
	}); err != nil {
		t.Fatalf("Done with --force should bypass tier gate: %v", err)
	}
}
