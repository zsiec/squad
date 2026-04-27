package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const attestTestItemBody = `---
id: FEAT-001
title: Attest test fixture
type: feature
priority: P1
area: core
status: open
created: 2026-04-25
updated: 2026-04-25
evidence_required: [test]
---

## Acceptance criteria
- [ ] the rule does the thing as specified
`

const attestReviewItemBody = `---
id: FEAT-001
title: Attest review fixture
type: feature
priority: P1
area: core
status: open
created: 2026-04-25
updated: 2026-04-25
evidence_required: [review]
---

## Acceptance criteria
- [ ] the rule does the thing as specified
`

func TestParseReviewExit_StopsAtSeparator(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"clean", "status: clean\n---\nbody\n", 0},
		{"blocking", "status: blocking\n---\nbody\n", 1},
		{"clean header, blocking line in body", "status: clean\ndisagreements: 0\n---\nprior reviewer wrote:\nstatus: blocking\nbut it was resolved.\n", 0},
		{"blocking header, clean line in body", "status: blocking\ndisagreements: 2\n---\nfinding 1:\nstatus: clean\nwas the prior verdict.\n", 1},
		{"no separator, blocking present", "status: blocking\nfollow-up notes\n", 1},
		{"no header at all", "no status header\n---\nbody\n", 0},
		{"missing header, body quotes status: blocking", "disagreements: 0\nresolution: accepted\n---\nfinding excerpt:\nstatus: blocking\n(now moot)\n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseReviewExit([]byte(tc.body))
			if got != tc.want {
				t.Fatalf("parseReviewExit(%q) = %d, want %d", tc.body, got, tc.want)
			}
		})
	}
}

func TestAttest_TestKind_HappyPath(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestTestItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "test", "--command", "printf 'ok\\n'"})
	if err := root.Execute(); err != nil {
		t.Fatalf("attest: %v\nout=%s", err, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "FEAT-001") {
		t.Errorf("output missing FEAT-001: %s", body)
	}
	if !strings.Contains(body, "test") {
		t.Errorf("output missing kind=test: %s", body)
	}

	attDir := filepath.Join(repoDir, ".squad", "attestations")
	entries, err := os.ReadDir(attDir)
	if err != nil {
		t.Fatalf("read attestations dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 attestation file, got %d", len(entries))
	}
}

func TestAttest_PositionalItemID(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-positional")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestTestItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Positional item id, no --item flag — matches the convention every
	// other claim verb (done, release, claim, blocked, etc.) uses.
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"attest", "FEAT-001", "--kind", "test", "--command", "printf 'ok\\n'"})
	if err := root.Execute(); err != nil {
		t.Fatalf("attest with positional: %v\nout=%s", err, out.String())
	}
	if !strings.Contains(out.String(), "FEAT-001") {
		t.Errorf("output missing FEAT-001: %s", out.String())
	}

	// Conflict between positional and --item should surface, not silently
	// pick one.
	conflict := newRootCmd()
	var cOut bytes.Buffer
	conflict.SetOut(&cOut)
	conflict.SetErr(&cOut)
	conflict.SetArgs([]string{"attest", "FEAT-001", "--item", "FEAT-002", "--kind", "test", "--command", "true"})
	if err := conflict.Execute(); err == nil {
		t.Errorf("expected error on positional/--item conflict; got none\nout=%s", cOut.String())
	}
}

func TestAttest_BadKind(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-2")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestTestItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "fabricated", "--command", "true"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid kind, got nil")
	}
	if !strings.Contains(err.Error(), "invalid kind") {
		t.Fatalf("error should contain 'invalid kind', got: %v", err)
	}
}

func TestAttest_Review_BlockingFindingsRecordsExit1(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-review-blocking")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestReviewItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	findings := filepath.Join(repoDir, "findings.md")
	body := "status: blocking\ndisagreements: 2\nresolution: rejected\n---\nfinding 1 narrative\nfinding 2 narrative\n"
	if err := os.WriteFile(findings, []byte(body), 0o644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"attest",
		"--item", "FEAT-001",
		"--kind", "review",
		"--reviewer-agent", "agent-r",
		"--findings-file", findings,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("attest: %v\nout=%s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "exit=1") {
		t.Errorf("output missing exit=1: %s", got)
	}
	if !strings.Contains(got, "FEAT-001") {
		t.Errorf("output missing FEAT-001: %s", got)
	}
}

// TestAttest_Review_BlockingCreatesGotchaLearning verifies a blocking
// review attestation auto-files a `learning_propose` of kind=gotcha
// with the rejection narrative as the body, tagged `review-rejection`,
// and the item linked via related_items. Closes the
// observation→knowledge gap by routing reviewer rationale into the
// learning ledger automatically.
func TestAttest_Review_BlockingCreatesGotchaLearning(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-review-learning-1")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestReviewItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	const narrative = "the helper accepts a slice but should accept a single value; rewriting the call sites is a one-line change"
	findings := filepath.Join(repoDir, "findings.md")
	body := "status: blocking\ndisagreements: 1\nresolution: rejected\n---\n" + narrative + "\n"
	if err := os.WriteFile(findings, []byte(body), 0o644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "review", "--reviewer-agent", "agent-r", "--findings-file", findings})
	if err := root.Execute(); err != nil {
		t.Fatalf("attest: %v\nout=%s", err, out.String())
	}

	gotchaDir := filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed")
	entries, err := os.ReadDir(gotchaDir)
	if err != nil {
		t.Fatalf("read gotchas/proposed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 proposed gotcha learning, got %d entries: %+v", len(entries), entries)
	}
	stub, err := os.ReadFile(filepath.Join(gotchaDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		narrative,
		"## Looks like",
		"FEAT-001",
		"review-rejection",
	} {
		if !strings.Contains(string(stub), want) {
			t.Errorf("gotcha stub missing %q\n---\n%s", want, stub)
		}
	}
	if strings.Contains(string(stub), "second-round") {
		t.Errorf("first rejection should not be tagged second-round:\n%s", stub)
	}
}

// TestAttest_Review_SecondBlockingTagsSecondRound verifies a second
// blocking review on the same item tags its learning `second-round` so
// triage queries can prioritize patterns biting twice.
func TestAttest_Review_SecondBlockingTagsSecondRound(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-review-learning-2")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"), []byte(attestReviewItemBody), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	mustReview := func(idx int, narrative string) {
		t.Helper()
		findings := filepath.Join(repoDir, fmt.Sprintf("findings-%d.md", idx))
		body := "status: blocking\nresolution: rejected\n---\n" + narrative + "\n"
		if err := os.WriteFile(findings, []byte(body), 0o644); err != nil {
			t.Fatalf("write findings %d: %v", idx, err)
		}
		c := newRootCmd()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "review", "--reviewer-agent", "agent-r", "--findings-file", findings})
		if err := c.Execute(); err != nil {
			t.Fatalf("attest %d: %v\nout=%s", idx, err, out.String())
		}
	}
	mustReview(1, "first rejection narrative — call sites use the wrong helper")
	mustReview(2, "second rejection narrative — same pattern, different file")

	gotchaDir := filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed")
	entries, err := os.ReadDir(gotchaDir)
	if err != nil {
		t.Fatalf("read gotchas/proposed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 proposed gotcha learnings (one per rejection), got %d", len(entries))
	}
	var secondRoundCount, firstRoundCount int
	for _, e := range entries {
		stub, err := os.ReadFile(filepath.Join(gotchaDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(stub), "second-round") {
			secondRoundCount++
		} else {
			firstRoundCount++
		}
	}
	if firstRoundCount != 1 || secondRoundCount != 1 {
		t.Errorf("want 1 first-round + 1 second-round, got first=%d second=%d", firstRoundCount, secondRoundCount)
	}
}

// TestAttest_Review_CleanDoesNotCreateLearning verifies a clean review
// (status: clean, exit=0) does NOT auto-create a learning — only
// blocking reviews trigger the pipeline.
func TestAttest_Review_CleanDoesNotCreateLearning(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-review-learning-3")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"), []byte(attestReviewItemBody), 0o644); err != nil {
		t.Fatalf("write item: %v", err)
	}
	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	findings := filepath.Join(repoDir, "findings.md")
	body := "status: clean\ndisagreements: 0\n---\nlooks good\n"
	if err := os.WriteFile(findings, []byte(body), 0o644); err != nil {
		t.Fatalf("write findings: %v", err)
	}
	root := newRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"attest", "--item", "FEAT-001", "--kind", "review", "--reviewer-agent", "agent-r", "--findings-file", findings})
	if err := root.Execute(); err != nil {
		t.Fatalf("attest: %v", err)
	}

	gotchaDir := filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed")
	entries, _ := os.ReadDir(gotchaDir)
	if len(entries) != 0 {
		t.Errorf("clean review should not create a learning; got %d entries: %+v", len(entries), entries)
	}
}

func TestAttest_Review_CleanFindingsRecordsExit0(t *testing.T) {
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("SQUAD_SESSION_ID", "test-attest-review-clean")
	t.Setenv("SQUAD_AGENT", "")
	gitInitDirCommittedMain(t, repoDir)
	t.Chdir(repoDir)

	initCmd := newInitCmd()
	initCmd.SetOut(&bytes.Buffer{})
	initCmd.SetErr(&bytes.Buffer{})
	initCmd.SetArgs([]string{"--yes", "--dir", repoDir})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-001-test.md"),
		[]byte(attestReviewItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}

	claim := newRootCmd()
	claim.SetOut(&bytes.Buffer{})
	claim.SetErr(&bytes.Buffer{})
	claim.SetArgs([]string{"claim", "FEAT-001"})
	if err := claim.Execute(); err != nil {
		t.Fatalf("claim: %v", err)
	}

	findings := filepath.Join(repoDir, "findings.md")
	body := "status: clean\ndisagreements: 0\nresolution: accepted\n---\nno blocking issues\n"
	if err := os.WriteFile(findings, []byte(body), 0o644); err != nil {
		t.Fatalf("write findings: %v", err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"attest",
		"--item", "FEAT-001",
		"--kind", "review",
		"--reviewer-agent", "agent-r",
		"--findings-file", findings,
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("attest: %v\nout=%s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "exit=0") {
		t.Errorf("output missing exit=0: %s", got)
	}
	if !strings.Contains(got, "FEAT-001") {
		t.Errorf("output missing FEAT-001: %s", got)
	}
}
