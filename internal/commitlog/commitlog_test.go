package commitlog

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

func TestRecordSinceClaim_CapturesInWindow(t *testing.T) {
	repo, db := newFixture(t)
	commitAt(t, repo, "before.go", "before claim", time.Now().Add(-1*time.Hour))
	claimedAt := time.Now().Add(-30 * time.Minute).Unix()
	commitAt(t, repo, "during1.go", "first during claim", time.Now().Add(-10*time.Minute))
	commitAt(t, repo, "during2.go", "second during claim", time.Now().Add(-5*time.Minute))

	n, err := RecordSinceClaim(context.Background(), db, "repo-test", repo, "BUG-501", "agent-a", claimedAt)
	if err != nil {
		t.Fatalf("RecordSinceClaim: %v", err)
	}
	if n != 2 {
		t.Fatalf("inserted=%d, want 2", n)
	}

	got, err := ListForItem(context.Background(), db, "repo-test", "BUG-501")
	if err != nil {
		t.Fatalf("ListForItem: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2: %+v", len(got), got)
	}
	subjects := map[string]bool{}
	for _, c := range got {
		subjects[c.Subject] = true
	}
	if !subjects["first during claim"] || !subjects["second during claim"] {
		t.Fatalf("subjects=%v want both 'first during claim' and 'second during claim'", subjects)
	}
}

func TestRecordSinceClaim_IsIdempotent(t *testing.T) {
	repo, db := newFixture(t)
	claimedAt := time.Now().Add(-1 * time.Hour).Unix()
	commitAt(t, repo, "x.go", "single", time.Now().Add(-30*time.Minute))

	n1, err := RecordSinceClaim(context.Background(), db, "repo-test", repo, "BUG-502", "agent-a", claimedAt)
	if err != nil || n1 != 1 {
		t.Fatalf("first run: n=%d err=%v", n1, err)
	}
	n2, err := RecordSinceClaim(context.Background(), db, "repo-test", repo, "BUG-502", "agent-a", claimedAt)
	if err != nil || n2 != 0 {
		t.Fatalf("second run must insert 0; got n=%d err=%v", n2, err)
	}
}

func TestRecordSinceClaim_EmptyRepoRootIsNoOp(t *testing.T) {
	_, db := newFixture(t)
	n, err := RecordSinceClaim(context.Background(), db, "repo-test", "", "BUG-503", "agent-a", 0)
	if err != nil || n != 0 {
		t.Fatalf("empty repoRoot should be no-op; got n=%d err=%v", n, err)
	}
}

func TestRecordSinceClaim_NoCommitsInWindow(t *testing.T) {
	repo, db := newFixture(t)
	commitAt(t, repo, "old.go", "way before", time.Now().Add(-2*time.Hour))
	claimedAt := time.Now().Add(-30 * time.Minute).Unix()

	n, err := RecordSinceClaim(context.Background(), db, "repo-test", repo, "BUG-504", "agent-a", claimedAt)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("inserted=%d, want 0 (commit pre-dates claim)", n)
	}
}

// TestListForItem_WorkspaceModeAggregatesAllRepos pins the "" sentinel:
// when the caller passes repoID == "", ListForItem must return rows for
// the given item id from every repo, not silently filter to repo_id = ''.
// The dashboard's links handler takes this path in workspace mode.
func TestListForItem_WorkspaceModeAggregatesAllRepos(t *testing.T) {
	repoA, db := newFixture(t)
	repoB := t.TempDir()
	for _, dir := range []string{repoB} {
		run := func(args ...string) {
			t.Helper()
			c := exec.Command("git", append([]string{"-C", dir}, args...)...)
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		run("init", "-q", "-b", "main")
		run("config", "user.email", "test@example.com")
		run("config", "user.name", "Test")
		run("config", "commit.gpgsign", "false")
	}
	claimedAt := time.Now().Add(-30 * time.Minute).Unix()
	commitAt(t, repoA, "a.go", "from-A", time.Now().Add(-10*time.Minute))
	commitAt(t, repoB, "b.go", "from-B", time.Now().Add(-5*time.Minute))

	if _, err := RecordSinceClaim(context.Background(), db, "repo-A", repoA, "BUG-700", "agent-a", claimedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordSinceClaim(context.Background(), db, "repo-B", repoB, "BUG-700", "agent-b", claimedAt); err != nil {
		t.Fatal(err)
	}

	got, err := ListForItem(context.Background(), db, "", "BUG-700")
	if err != nil {
		t.Fatalf("ListForItem(repoID=\"\"): %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("workspace-mode listing returned %d rows, want 2 (cross-repo): %+v", len(got), got)
	}
}

// --- fixtures ---

func newFixture(t *testing.T) (string, *sql.DB) {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", repo}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	db, err := store.Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return repo, db
}

func commitAt(t *testing.T, repo, file, subject string, at time.Time) {
	t.Helper()
	if file != "" {
		w := exec.Command("sh", "-c", fmt.Sprintf("echo %q >> %s", subject, file))
		w.Dir = repo
		if out, err := w.CombinedOutput(); err != nil {
			t.Fatalf("write: %v\n%s", err, out)
		}
		add := exec.Command("git", "-C", repo, "add", file)
		if out, err := add.CombinedOutput(); err != nil {
			t.Fatalf("add: %v\n%s", err, out)
		}
	}
	ts := at.Format(time.RFC3339)
	c := exec.Command("git", "-C", repo, "commit", "--allow-empty", "-m", subject, "--date", ts)
	c.Env = append(c.Environ(), "GIT_AUTHOR_DATE="+ts, "GIT_COMMITTER_DATE="+ts)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
}
