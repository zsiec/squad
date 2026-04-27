package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/hygiene"
	"github.com/zsiec/squad/internal/learning"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
	"github.com/zsiec/squad/plugin/hooks"
)

// brokenItemBody — frontmatter blockedBy a missing item to trip a hygiene
// finding deterministically.
const brokenItemBody = `---
id: FEAT-901
title: Broken-ref test item
type: feat
status: filed
priority: P3
created: 2026-04-26
updated: 2026-04-26
blocked-by: ["FEAT-9999-does-not-exist"]
---
## Problem
seed
`

// setupDoctorRepoBare initialises a repo + isolated SQUAD_HOME and returns
// (repoDir, run). It does NOT seed the FEAT-901 broken item — callers that
// rely on a pre-existing finding should use setupDoctorRepo instead.
func setupDoctorRepoBare(t *testing.T) (string, func(args ...string) (string, error)) {
	t.Helper()
	repoDir := t.TempDir()
	state := t.TempDir()
	t.Setenv("SQUAD_HOME", state)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("SQUAD_SESSION_ID", "test-doctor-"+t.Name())
	t.Setenv("SQUAD_AGENT", "")
	gitInitDir(t, repoDir)
	t.Chdir(repoDir)

	run := func(args ...string) (string, error) {
		var c *cobra.Command
		if args[0] == "init" {
			c = newInitCmd()
			args = args[1:]
		} else {
			c = newRootCmd()
		}
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetErr(&out)
		c.SetArgs(args)
		err := c.Execute()
		return out.String(), err
	}

	if out, err := run("init", "--yes", "--dir", repoDir); err != nil {
		t.Fatalf("init: %v\nout=%s", err, out)
	}
	return repoDir, run
}

func setupDoctorRepo(t *testing.T) func(args ...string) (string, error) {
	t.Helper()
	repoDir, run := setupDoctorRepoBare(t)
	if err := os.WriteFile(
		filepath.Join(repoDir, ".squad", "items", "FEAT-901-broken.md"),
		[]byte(brokenItemBody),
		0o644,
	); err != nil {
		t.Fatalf("write item: %v", err)
	}
	return run
}

func TestDoctor_ExitsZeroOnFindingsByDefault(t *testing.T) {
	run := setupDoctorRepo(t)

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("expected exit 0 even with findings; got err=%v\nout=%s", err, out)
	}
	if !strings.Contains(out, "finding(s)") {
		t.Errorf("doctor stdout missing 'finding(s)': %s", out)
	}
}

func TestDoctor_StrictReturnsErrorOnFindings(t *testing.T) {
	run := setupDoctorRepo(t)

	out, err := run("doctor", "--strict")
	if err == nil {
		t.Fatalf("expected --strict to return error when findings present\nout=%s", out)
	}
	if !strings.Contains(out, "finding(s)") {
		t.Errorf("doctor stdout missing 'finding(s)': %s", out)
	}
}

func materializeAllHooksFor(t *testing.T, hookDir string) {
	t.Helper()
	if err := os.MkdirAll(hookDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	entries, err := fs.ReadDir(hooks.FS, ".")
	if err != nil {
		t.Fatalf("read embed: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sh" {
			continue
		}
		body, err := fs.ReadFile(hooks.FS, e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(hookDir, e.Name()), body, 0o755); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}
}

func TestDoctor_ReportsHookDriftFromPluginRoot(t *testing.T) {
	run := setupDoctorRepo(t)

	pluginRoot := t.TempDir()
	hookDir := filepath.Join(pluginRoot, "hooks")
	materializeAllHooksFor(t, hookDir)
	if err := os.WriteFile(filepath.Join(hookDir, "session_start.sh"), []byte("# tampered\n"), 0o755); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", pluginRoot)

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "session_start.sh") || !strings.Contains(out, "modified") {
		t.Errorf("doctor stdout missing hook drift line: %s", out)
	}
}

func TestDoctor_StrictFailsOnHookDrift(t *testing.T) {
	run := setupDoctorRepo(t)

	pluginRoot := t.TempDir()
	hookDir := filepath.Join(pluginRoot, "hooks")
	materializeAllHooksFor(t, hookDir)
	if err := os.WriteFile(filepath.Join(hookDir, "session_start.sh"), []byte("# tampered\n"), 0o755); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", pluginRoot)

	out, err := run("doctor", "--strict")
	if err == nil {
		t.Fatalf("expected --strict to fail when hook drift present\nout=%s", out)
	}
}

// seedCapturedItem inserts a captured item directly into the global db for
// the test repo. Bypasses item-file authoring on purpose: the doctor checks
// query items by status/captured_at and don't need the on-disk file.
func seedCapturedItem(t *testing.T, repoDir, itemID string, capturedAt int64) {
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
	if _, err := db.Exec(`
		INSERT INTO items (repo_id, item_id, title, type, priority, status, path, updated_at, captured_at)
		VALUES (?, ?, ?, 'feat', 'P3', 'captured', ?, ?, ?)
	`, repoID, itemID, itemID+" title",
		filepath.Join(".squad", "items", itemID+".md"),
		time.Now().Unix(), capturedAt); err != nil {
		t.Fatalf("seed item %s: %v", itemID, err)
	}
}

func TestDoctor_FlagsStaleCapture(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	seedCapturedItem(t, repoDir, "FEAT-OLD", old)

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "stale_capture") || !strings.Contains(out, "FEAT-OLD") {
		t.Fatalf("expected stale_capture finding for FEAT-OLD; got:\n%s", out)
	}
}

func TestDoctor_StrictFailsOnStaleCapture(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	seedCapturedItem(t, repoDir, "FEAT-OLD", old)

	out, err := run("doctor", "--strict")
	if err == nil {
		t.Fatalf("expected --strict to fail on stale capture\nout=%s", out)
	}
}

func TestDoctor_FlagsInboxOverflow(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	now := time.Now().Unix()
	for i := 0; i < 51; i++ {
		seedCapturedItem(t, repoDir, fmt.Sprintf("FEAT-%03d", i), now)
	}

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "inbox_overflow") {
		t.Fatalf("expected inbox_overflow finding; got:\n%s", out)
	}
}

// TestProposeDoctorLearnings_FirstSweepWritesArtifact pins that a
// findings list with a single kind produces exactly one proposed
// gotcha learning under .squad/learnings/gotchas/proposed/, tagged so
// future sweeps can detect the prior emission and debounce.
func TestProposeDoctorLearnings_FirstSweepWritesArtifact(t *testing.T) {
	repoDir, _ := setupDoctorRepoBare(t)

	findings := []hygiene.Finding{
		{Severity: hygiene.SeverityWarn, Code: "stale_claim", Message: "stale claim: BUG-001 (agent=agent-z)", Fix: "squad force-release BUG-001 --reason \"stale\""},
		{Severity: hygiene.SeverityWarn, Code: "stale_claim", Message: "stale claim: BUG-002 (agent=agent-z)", Fix: "squad force-release BUG-002 --reason \"stale\""},
	}
	proposeDoctorLearnings(context.Background(), repoDir, findings)

	matches, _ := filepath.Glob(filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed", "doctor-stale-claim-*.md"))
	if len(matches) != 1 {
		t.Fatalf("want 1 proposed doctor learning, got %d (%v)", len(matches), matches)
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"stale claim: BUG-001",
		"stale claim: BUG-002",
		"force-release",
		"doctor-finding",
		"doctor-code-stale-claim",
		"## Looks like",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("artifact missing %q\n---\n%s", want, body)
		}
	}
}

// TestProposeDoctorLearnings_DebouncedWithin7Days pins that a second
// sweep with the same code does NOT spawn a second artifact when the
// first one is still inside the debounce window.
func TestProposeDoctorLearnings_DebouncedWithin7Days(t *testing.T) {
	repoDir, _ := setupDoctorRepoBare(t)

	finding := []hygiene.Finding{{Severity: hygiene.SeverityWarn, Code: "stale_claim", Message: "stale: A", Fix: "force-release"}}
	proposeDoctorLearnings(context.Background(), repoDir, finding)
	proposeDoctorLearnings(context.Background(), repoDir, finding)

	matches, _ := filepath.Glob(filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed", "doctor-stale-claim-*.md"))
	if len(matches) != 1 {
		t.Fatalf("debounce broken: want 1 artifact across two sweeps, got %d (%v)", len(matches), matches)
	}
}

// TestProposeDoctorLearnings_ArtifactRoundTripsThroughLearningWalk
// pins that the emitted artifact is YAML-parseable so the debounce
// check (which queries learning.Walk for prior tagged proposals)
// can actually see prior emissions. A title with a `: ` substring
// would silently break this — Walk would drop the file and every
// sweep would re-emit with no debounce visible.
func TestProposeDoctorLearnings_ArtifactRoundTripsThroughLearningWalk(t *testing.T) {
	repoDir, _ := setupDoctorRepoBare(t)
	proposeDoctorLearnings(context.Background(), repoDir,
		[]hygiene.Finding{{Severity: hygiene.SeverityWarn, Code: "stale_claim", Message: "stale: A", Fix: "force-release"}},
	)
	walked, err := learning.Walk(repoDir)
	if err != nil {
		t.Fatalf("learning.Walk: %v", err)
	}
	var found *learning.Learning
	for i := range walked {
		if hasTag(walked[i].Tags, "doctor-finding") {
			found = &walked[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("learning.Walk did not surface the doctor artifact (parse failure?); walked=%+v", walked)
	}
	if found.Slug == "" {
		t.Errorf("walked learning has empty Slug; parse likely partial: %+v", found)
	}
	if !hasTag(found.Tags, "doctor-code-stale-claim") {
		t.Errorf("walked learning missing doctor-code-stale-claim tag: %+v", found.Tags)
	}
}

// TestProposeDoctorLearnings_ReEmitsAfter7Days pins that the debounce
// window is BOUNDED — once the prior artifact's Created date drops
// outside the 7-day window, a fresh sweep re-emits. Without the
// YAML-parseable artifact (above), this would never trigger because
// learning.Walk would skip the prior file entirely.
func TestProposeDoctorLearnings_ReEmitsAfter7Days(t *testing.T) {
	repoDir, _ := setupDoctorRepoBare(t)

	finding := []hygiene.Finding{{Severity: hygiene.SeverityWarn, Code: "stale_claim", Message: "stale: A", Fix: "force-release"}}
	proposeDoctorLearnings(context.Background(), repoDir, finding)

	// Backdate the artifact to 8 days ago so the next sweep's debounce
	// window (now - 7d) falls AFTER the artifact's Created. The cleanest
	// way to do this without injecting a clock everywhere is to rewrite
	// the file's `created:` line directly — same shape the test fixture
	// approach uses elsewhere in this package.
	matches, _ := filepath.Glob(filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed", "doctor-stale-claim-*.md"))
	if len(matches) != 1 {
		t.Fatalf("setup: want 1 artifact, got %d", len(matches))
	}
	today := time.Now().UTC().Format("2006-01-02")
	old := time.Now().UTC().AddDate(0, 0, -8).Format("2006-01-02")
	body, _ := os.ReadFile(matches[0])
	patched := strings.NewReplacer(
		"created: "+today, "created: "+old,
		// Also retire the slug so today's emission's slug doesn't
		// collide on the in-tree walk; without this the second sweep
		// hits SlugCollisionError, swallows it, and writes nothing —
		// masking the real "did debounce expire?" check.
		"slug: doctor-stale-claim-"+today, "slug: doctor-stale-claim-"+old,
	).Replace(string(body))
	if patched == string(body) {
		t.Fatalf("setup: failed to backdate created/slug in %s", matches[0])
	}
	// Slug + filename derive from the same string in stubBody, so to
	// keep them in sync the patched file also moves to the old-dated
	// path; otherwise today's emission would WriteFile the new content
	// to the same path as the patched file and silently overwrite it.
	oldPath := filepath.Join(filepath.Dir(matches[0]), "doctor-stale-claim-"+old+".md")
	if err := os.WriteFile(oldPath, []byte(patched), 0o644); err != nil {
		t.Fatalf("write old-dated: %v", err)
	}
	if err := os.Remove(matches[0]); err != nil {
		t.Fatalf("remove original: %v", err)
	}

	proposeDoctorLearnings(context.Background(), repoDir, finding)

	after, _ := filepath.Glob(filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed", "doctor-stale-claim-*.md"))
	if len(after) != 2 {
		t.Errorf("after backdating prior artifact past 7d window, want 2 artifacts (debounce expired); got %d (%v)", len(after), after)
	}
}

// TestDoctor_NoLearningsFlagSuppresses pins that the --no-learnings
// flag short-circuits the auto-emit so CI / scripted audits get the
// pure diagnostic output without a side effect.
func TestDoctor_NoLearningsFlagSuppresses(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	old := time.Now().Add(-60 * 24 * time.Hour).Unix()
	seedCapturedItem(t, repoDir, "FEAT-OLD", old)

	out, err := run("doctor", "--no-learnings")
	if err != nil {
		t.Fatalf("doctor --no-learnings: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "stale_capture") {
		t.Fatalf("doctor should still print findings under --no-learnings; out=%s", out)
	}
	matches, _ := filepath.Glob(filepath.Join(repoDir, ".squad", "learnings", "gotchas", "proposed", "doctor-*.md"))
	if len(matches) != 0 {
		t.Errorf("--no-learnings should not write any doctor artifact; got %d (%v)", len(matches), matches)
	}
}

func TestDoctor_FlagsRejectedLogOverflow(t *testing.T) {
	repoDir, run := setupDoctorRepoBare(t)
	var b strings.Builder
	for i := 0; i < 501; i++ {
		fmt.Fprintf(&b, "rejection %d\n", i)
	}
	logPath := filepath.Join(repoDir, ".squad", "rejected.log")
	if err := os.WriteFile(logPath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write rejected.log: %v", err)
	}

	out, err := run("doctor")
	if err != nil {
		t.Fatalf("doctor: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "rejected_log_overflow") {
		t.Fatalf("expected rejected_log_overflow finding; got:\n%s", out)
	}
}
