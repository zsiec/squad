package postmortem

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, cmd := range [][]string{
		{"git", "init", "-q"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "config", "commit.gpgsign", "false"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v: %s", cmd, err, out)
		}
	}
}

func gitCommit(t *testing.T, dir, path, content, msg string, when time.Time) {
	t.Helper()
	full := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	add := exec.Command("git", "add", path)
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	commit := exec.Command("git", "commit", "-q", "-m", msg)
	commit.Dir = dir
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+when.Format(time.RFC3339),
		"GIT_COMMITTER_DATE="+when.Format(time.RFC3339),
	)
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}

func setupFixture(t *testing.T) (dir, itemPath string) {
	t.Helper()
	dir = t.TempDir()
	gitInit(t, dir)
	itemRel := filepath.Join(".squad", "items", "FEAT-X.md")
	// Commit the initial file 24h before the default claim window so
	// it does NOT register as an in-window edit. Tests that want an
	// in-window edit add their own commit at window-midpoint.
	gitCommit(t, dir, itemRel, "# initial\n", "initial item", time.Now().Add(-24*time.Hour))
	return dir, filepath.Join(dir, itemRel)
}

func defaultOpts(db *sql.DB, itemPath, repoRoot string) Opts {
	now := time.Now().Unix()
	return Opts{
		DB:         db,
		RepoID:     "repo-1",
		ItemID:     "FEAT-X",
		AgentID:    "agent-test",
		ClaimedAt:  now - 7200,
		ReleasedAt: now,
		RepoRoot:   repoRoot,
		ItemPath:   itemPath,
		Enabled:    true,
	}
}

// AC#5: release with NO item-file delta, NO learning artifact, NO
// chat posts → detector returns "dispatch".
func TestDetect_NoArtifactsDispatches(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	d, err := Detect(context.Background(), defaultOpts(db, itemPath, dir))
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !d.Dispatch {
		t.Errorf("expected Dispatch=true, got %+v", d)
	}
	if d.Reason == "" {
		t.Errorf("Reason should explain why dispatch fires")
	}
}

// AC#6: claim window with a `## Premise audit` section added to the
// item file → detector skips dispatch.
func TestDetect_ItemFileEditedSkips(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)
	// Add a commit during the claim window.
	gitCommit(t, dir, filepath.Join(".squad", "items", "FEAT-X.md"),
		"# initial\n\n## Premise audit\n\nDetailed analysis of why the trigger doesn't fire.\n",
		"add premise audit",
		time.Unix((opts.ClaimedAt+opts.ReleasedAt)/2, 0))
	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if d.Dispatch {
		t.Errorf("expected Dispatch=false (item-file edited in window), got %+v", d)
	}
	found := false
	for _, s := range d.Signals {
		if s.Kind == SignalItemFileEdit {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SignalItemFileEdit in signals, got %+v", d.Signals)
	}
}

// AC#7: ≥`min_chat_messages` substantive chat posts by the claimant
// on the item thread → detector skips dispatch.
func TestDetect_SubstantiveChatSkips(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)

	for i := 0; i < 3; i++ {
		_, err := db.Exec(`INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body)
			VALUES (?, ?, ?, ?, 'fyi', ?)`,
			opts.RepoID, opts.ClaimedAt+int64(60*(i+1)), opts.AgentID, opts.ItemID,
			strings.Repeat("a substantive message body for signal detection. ", 3))
		if err != nil {
			t.Fatal(err)
		}
	}

	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if d.Dispatch {
		t.Errorf("expected Dispatch=false (3 substantive posts), got %+v", d)
	}
	found := false
	for _, s := range d.Signals {
		if s.Kind == SignalSubstantiveChat {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SignalSubstantiveChat in signals, got %+v", d.Signals)
	}
}

// Short messages do not count toward the substantive-chat threshold.
func TestDetect_ShortChatDoesNotSuppress(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)
	for i := 0; i < 5; i++ {
		_, err := db.Exec(`INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body)
			VALUES (?, ?, ?, ?, 'fyi', 'ok')`,
			opts.RepoID, opts.ClaimedAt+int64(60*(i+1)), opts.AgentID, opts.ItemID)
		if err != nil {
			t.Fatal(err)
		}
	}
	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !d.Dispatch {
		t.Errorf("expected Dispatch=true (only short messages), got %+v", d)
	}
}

// AC#8: `enabled: false` short-circuits the detector regardless of
// signal presence.
func TestDetect_DisabledShortCircuits(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)
	opts.Enabled = false

	// Add many signals — none should matter.
	gitCommit(t, dir, filepath.Join(".squad", "items", "FEAT-X.md"),
		"# initial\n\n## Premise audit\nbody.\n",
		"edit",
		time.Unix((opts.ClaimedAt+opts.ReleasedAt)/2, 0))
	for i := 0; i < 5; i++ {
		_, _ = db.Exec(`INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body)
			VALUES (?, ?, ?, ?, 'fyi', ?)`,
			opts.RepoID, opts.ClaimedAt+int64(60*(i+1)), opts.AgentID, opts.ItemID,
			strings.Repeat("substantive ", 10))
	}

	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if d.Dispatch {
		t.Errorf("disabled config must short-circuit to Dispatch=false, got %+v", d)
	}
	if len(d.Signals) != 1 || d.Signals[0].Kind != SignalDisabled {
		t.Errorf("disabled run should record one SignalDisabled, got %+v", d.Signals)
	}
}

// Edge case: a learning artifact filed (and committed) during the
// claim window suppresses dispatch even when no other signals exist.
// Uses git log, not mtime, so the test must commit the file inside
// the window for the detector to see it.
func TestDetect_LearningArtifactSkips(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)

	mid := time.Unix((opts.ClaimedAt+opts.ReleasedAt)/2, 0)
	gitCommit(t, dir,
		filepath.Join(".squad", "learnings", "proposed", "test-lesson.md"),
		"---\nstate: proposed\n---\n",
		"propose lesson",
		mid)

	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if d.Dispatch {
		t.Errorf("expected Dispatch=false (learning artifact in window), got %+v", d)
	}
	found := false
	for _, s := range d.Signals {
		if s.Kind == SignalLearningArtifact {
			found = true
		}
	}
	if !found {
		t.Errorf("expected SignalLearningArtifact, got %+v", d.Signals)
	}
}

// TestDetect_UntrackedLearningSkips verifies an in-progress proposal
// (filed but not committed yet) still suppresses dispatch — the
// claimant is mid-flight on lesson capture, dispatching another
// agent would clobber.
func TestDetect_UntrackedLearningSkips(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)

	learningDir := filepath.Join(dir, ".squad", "learnings", "proposed")
	if err := os.MkdirAll(learningDir, 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(learningDir, "in-flight.md")
	if err := os.WriteFile(artifact, []byte("---\nstate: proposed\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mid := time.Unix((opts.ClaimedAt+opts.ReleasedAt)/2, 0)
	if err := os.Chtimes(artifact, mid, mid); err != nil {
		t.Fatal(err)
	}

	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if d.Dispatch {
		t.Errorf("expected Dispatch=false (untracked artifact in window), got %+v", d)
	}
}

// TestDetect_PeerChatSuppresses: peer-authored substantive chat on
// the item thread DOES suppress dispatch. The question is whether
// the durable record exists, not who wrote it.
func TestDetect_PeerChatSuppresses(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)
	for i := 0; i < 3; i++ {
		_, err := db.Exec(`INSERT INTO messages (repo_id, ts, agent_id, thread, kind, body)
			VALUES (?, ?, ?, ?, 'fyi', ?)`,
			opts.RepoID, opts.ClaimedAt+int64(60*(i+1)),
			"agent-peer-not-claimant", opts.ItemID,
			strings.Repeat("a peer's analysis fully captures the lesson durably. ", 3))
		if err != nil {
			t.Fatal(err)
		}
	}
	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if d.Dispatch {
		t.Errorf("peer-authored substantive chat should suppress dispatch, got %+v", d)
	}
}

// TestDetect_ExistingPostmortemSkips: rerun is idempotent — if a
// learning artifact named "<itemID>-postmortem-*" already exists,
// the detector short-circuits regardless of other signals.
func TestDetect_ExistingPostmortemSkips(t *testing.T) {
	dir, itemPath := setupFixture(t)
	db := openTestDB(t)
	opts := defaultOpts(db, itemPath, dir)

	learningDir := filepath.Join(dir, ".squad", "learnings", "proposed")
	if err := os.MkdirAll(learningDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := filepath.Join(learningDir, opts.ItemID+"-postmortem-20260101-000000.md")
	if err := os.WriteFile(prior, []byte("---\nstate: proposed\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := Detect(context.Background(), opts)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if d.Dispatch {
		t.Errorf("existing postmortem must short-circuit, got %+v", d)
	}
	if len(d.Signals) != 1 || d.Signals[0].Kind != SignalAlreadyRan {
		t.Errorf("expected single SignalAlreadyRan, got %+v", d.Signals)
	}
}
