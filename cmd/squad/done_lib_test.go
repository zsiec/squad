package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/config"
)

func TestDone_PureClosesClaimedItem(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-300")
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-300", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	res, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-300", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if res == nil || res.ItemID != "BUG-300" || res.ForceOverride {
		t.Fatalf("unexpected result: %+v", res)
	}
}

const evidenceItem = `---
id: BUG-301
title: needs evidence
type: bug
priority: P1
area: core
status: open
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test, review]
---

## Acceptance criteria
- [ ] x
`

func TestDone_PureRejectsMissingEvidence(t *testing.T) {
	env := newTestEnv(t)
	itemPath := filepath.Join(env.ItemsDir, "BUG-301.md")
	if err := os.WriteFile(itemPath, []byte(evidenceItem), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-301", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	_, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-301", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	var miss *EvidenceMissingError
	if !errors.As(err, &miss) {
		t.Fatalf("err=%v want *EvidenceMissingError", err)
	}
	if len(miss.Missing) != 2 {
		t.Fatalf("want 2 missing, got %v", miss.Missing)
	}
}

func TestDone_PureForceRecordsOverride(t *testing.T) {
	env := newTestEnv(t)
	itemPath := filepath.Join(env.ItemsDir, "BUG-302.md")
	if err := os.WriteFile(itemPath,
		[]byte(strings.ReplaceAll(evidenceItem, "BUG-301", "BUG-302")), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-302", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	res, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-302", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root, Force: true,
	})
	if err != nil {
		t.Fatalf("Done force: %v", err)
	}
	if !res.ForceOverride || len(res.BypassedKinds) != 2 {
		t.Fatalf("unexpected result: %+v", res)
	}
	L := attest.New(env.DB, env.RepoID, nil)
	recs, err := L.ListForItem(context.Background(), "BUG-302")
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Kind != attest.KindManual {
		t.Fatalf("want 1 manual attestation, got %+v", recs)
	}
}

func TestDone_PureSatisfiedEvidenceProceeds(t *testing.T) {
	env := newTestEnv(t)
	itemPath := filepath.Join(env.ItemsDir, "BUG-303.md")
	if err := os.WriteFile(itemPath,
		[]byte(strings.NewReplacer("BUG-301", "BUG-303", "evidence_required: [test, review]", "evidence_required: [test]").Replace(evidenceItem)),
		0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-303", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	L := attest.New(env.DB, env.RepoID, nil)
	if _, err := L.Insert(context.Background(), attest.Record{
		ItemID: "BUG-303", Kind: attest.KindTest, Command: "go test", ExitCode: 0,
		OutputHash: "deadbeef", OutputPath: "/dev/null", AgentID: env.AgentID,
	}); err != nil {
		t.Fatal(err)
	}

	res, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-303", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	if err != nil {
		t.Fatalf("Done: %v", err)
	}
	if res.ForceOverride {
		t.Fatalf("force should be false: %+v", res)
	}
}

func TestDone_PureFallsBackToDefaultEvidenceRequired(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-310")
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-310", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	_, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-310", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot:                env.Root,
		DefaultEvidenceRequired: []string{"test"},
	})
	var miss *EvidenceMissingError
	if !errors.As(err, &miss) {
		t.Fatalf("err=%v want *EvidenceMissingError", err)
	}
	if len(miss.Missing) != 1 || miss.Missing[0] != attest.KindTest {
		t.Fatalf("Missing=%v want [test]", miss.Missing)
	}
}

func TestDone_RoundTripsConfigDefaultsThroughDone(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-312")
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-312", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(env.Root, ".squad", "config.yaml"),
		[]byte("defaults:\n  evidence_required: [test]\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(env.Root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	_, err = Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-312", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot:                env.Root,
		DefaultEvidenceRequired: cfg.Defaults.EvidenceRequired,
	})
	var miss *EvidenceMissingError
	if !errors.As(err, &miss) {
		t.Fatalf("err=%v want *EvidenceMissingError", err)
	}
	if len(miss.Missing) != 1 || miss.Missing[0] != attest.KindTest {
		t.Fatalf("Missing=%v want [test]", miss.Missing)
	}
}

func TestDone_PerItemEvidenceWinsOverDefault(t *testing.T) {
	env := newTestEnv(t)
	itemPath := filepath.Join(env.ItemsDir, "BUG-311.md")
	body := strings.ReplaceAll(evidenceItem, "BUG-301", "BUG-311")
	if err := os.WriteFile(itemPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Claim(context.Background(), ClaimArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-311", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
	}); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	_, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-311", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot:                env.Root,
		DefaultEvidenceRequired: []string{"manual"},
	})
	var miss *EvidenceMissingError
	if !errors.As(err, &miss) {
		t.Fatalf("err=%v want *EvidenceMissingError", err)
	}
	if len(miss.Missing) != 2 {
		t.Fatalf("want 2 missing (per-item test+review), got %v", miss.Missing)
	}
	for _, k := range miss.Missing {
		if k == attest.KindManual {
			t.Fatalf("default 'manual' leaked into per-item-driven Missing: %v", miss.Missing)
		}
	}
}

func TestDone_PureRejectsNotYours(t *testing.T) {
	env := newTestEnv(t)
	writeMinimalItem(t, env.ItemsDir, "BUG-304")
	if _, err := env.DB.Exec(`
		INSERT INTO claims (repo_id, item_id, agent_id, claimed_at, last_touch, intent, long)
		VALUES (?, ?, 'agent-other', 1000, 1000, '', 0)
	`, env.RepoID, "BUG-304"); err != nil {
		t.Fatal(err)
	}
	_, err := Done(context.Background(), DoneArgs{
		DB: env.DB, RepoID: env.RepoID, AgentID: env.AgentID,
		ItemID: "BUG-304", ItemsDir: env.ItemsDir, DoneDir: env.DoneDir,
		RepoRoot: env.Root,
	})
	if !errors.Is(err, claims.ErrNotYours) {
		t.Fatalf("err=%v want ErrNotYours", err)
	}
}
