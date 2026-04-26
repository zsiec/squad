package prmark

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"
)

func TestResolveCommits_HappyPath(t *testing.T) {
	repo := initFixtureRepo(t)
	makeCommit(t, repo, "first.go", "first")
	makeCommit(t, repo, "second.go", "second touched")
	makeCommit(t, repo, "third.go", "third")

	got, err := ResolveCommits(context.Background(), CommitQuery{
		RepoRoot:     repo,
		Branch:       "main",
		TouchedFiles: []string{"second.go"},
		Limit:        20,
	})
	if err != nil {
		t.Fatalf("ResolveCommits: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d commits, want 1: %+v", len(got), got)
	}
	if got[0].Subject != "second touched" {
		t.Fatalf("subject=%q want=second touched", got[0].Subject)
	}
	if len(got[0].Sha) != 40 {
		t.Fatalf("sha=%q (len=%d), want full 40-char sha", got[0].Sha, len(got[0].Sha))
	}
}

func TestResolveCommits_RespectsLimit(t *testing.T) {
	repo := initFixtureRepo(t)
	for i := 0; i < 25; i++ {
		makeCommit(t, repo, "f.go", fmt.Sprintf("commit %d", i))
	}
	got, err := ResolveCommits(context.Background(), CommitQuery{
		RepoRoot:     repo,
		Branch:       "main",
		TouchedFiles: []string{"f.go"},
		Limit:        20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 20 {
		t.Fatalf("got %d commits, want 20", len(got))
	}
}

func TestResolveCommits_FiltersToTouchedFiles(t *testing.T) {
	repo := initFixtureRepo(t)
	makeCommit(t, repo, "a.go", "edited a")
	makeCommit(t, repo, "b.go", "edited b")
	makeCommit(t, repo, "c.go", "edited c")

	got, err := ResolveCommits(context.Background(), CommitQuery{
		RepoRoot:     repo,
		Branch:       "main",
		TouchedFiles: []string{"a.go", "c.go"},
		Limit:        20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d commits, want 2: %+v", len(got), got)
	}
}

func TestResolveCommits_SinceUntilWindow(t *testing.T) {
	repo := initFixtureRepo(t)
	makeCommitAt(t, repo, "f.go", "before window", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	makeCommitAt(t, repo, "f.go", "in window", time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC))
	makeCommitAt(t, repo, "f.go", "after window", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	since := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	got, err := ResolveCommits(context.Background(), CommitQuery{
		RepoRoot:     repo,
		Branch:       "main",
		TouchedFiles: []string{"f.go"},
		Since:        since,
		Until:        until,
		Limit:        20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Subject != "in window" {
		t.Fatalf("got=%+v want one commit with subject 'in window'", got)
	}
}

func TestResolveCommits_RejectsShellMetasInBranch(t *testing.T) {
	repo := initFixtureRepo(t)
	cases := []string{
		"main; rm -rf /",
		"main && echo pwned",
		"main`whoami`",
		"main$(id)",
		"main|cat /etc/passwd",
		"main\nrm",
		"-fsanitize=address",        // looks like a flag
		"main\x00null",
		"..",
		"feat/../../etc/passwd",
	}
	for _, branch := range cases {
		t.Run(branch, func(t *testing.T) {
			_, err := ResolveCommits(context.Background(), CommitQuery{
				RepoRoot:     repo,
				Branch:       branch,
				TouchedFiles: []string{"f.go"},
				Limit:        20,
			})
			if err == nil {
				t.Fatalf("branch=%q must be rejected, got nil error", branch)
			}
		})
	}
}

func TestResolveCommits_EmptyBranchReturnsError(t *testing.T) {
	repo := initFixtureRepo(t)
	_, err := ResolveCommits(context.Background(), CommitQuery{
		RepoRoot:     repo,
		Branch:       "",
		TouchedFiles: []string{"f.go"},
		Limit:        20,
	})
	if err == nil {
		t.Fatal("empty branch must error")
	}
}

func TestResolveCommits_UnknownBranchReturnsError(t *testing.T) {
	repo := initFixtureRepo(t)
	makeCommit(t, repo, "f.go", "x")
	_, err := ResolveCommits(context.Background(), CommitQuery{
		RepoRoot:     repo,
		Branch:       "branch-that-does-not-exist",
		TouchedFiles: []string{"f.go"},
		Limit:        20,
	})
	if err == nil {
		t.Fatal("unknown branch must error")
	}
}

// --- fixture helpers ---

func initFixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
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
	return dir
}

func makeCommit(t *testing.T, repo, file, subject string) {
	t.Helper()
	if file != "" {
		write := exec.Command("sh", "-c", fmt.Sprintf("echo %q >> %s", subject, file))
		write.Dir = repo
		if out, err := write.CombinedOutput(); err != nil {
			t.Fatalf("write %s: %v\n%s", file, err, out)
		}
		add := exec.Command("git", "-C", repo, "add", file)
		if out, err := add.CombinedOutput(); err != nil {
			t.Fatalf("add %s: %v\n%s", file, err, out)
		}
	}
	commit := exec.Command("git", "-C", repo, "commit", "--allow-empty", "-m", subject)
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("commit %q: %v\n%s", subject, err, out)
	}
}

func makeCommitAt(t *testing.T, repo, file, subject string, at time.Time) {
	t.Helper()
	ts := at.Format(time.RFC3339)
	if file != "" {
		write := exec.Command("sh", "-c", fmt.Sprintf("echo %q >> %s", subject, file))
		write.Dir = repo
		if out, err := write.CombinedOutput(); err != nil {
			t.Fatalf("write: %v\n%s", err, out)
		}
		add := exec.Command("git", "-C", repo, "add", file)
		if out, err := add.CombinedOutput(); err != nil {
			t.Fatalf("add: %v\n%s", err, out)
		}
	}
	commit := exec.Command("git", "-C", repo, "commit", "--allow-empty", "-m", subject, "--date", ts)
	commit.Env = append(commit.Environ(), "GIT_AUTHOR_DATE="+ts, "GIT_COMMITTER_DATE="+ts)
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("commit %q at %s: %v\n%s", subject, ts, err, out)
	}
}
