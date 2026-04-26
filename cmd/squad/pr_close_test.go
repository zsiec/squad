package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/store"
)

func fakeGH(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' \"" + strings.ReplaceAll(body, `"`, `\"`) + "\"\n"
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	return dir
}

func withFakeGH(t *testing.T, body string) {
	t.Helper()
	dir := fakeGH(t, body)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestPRCloseArchivesItem(t *testing.T) {
	repo := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())
	withFakeGH(t, "## Summary\n\nFixed it.\n\n<!-- squad-item: BUG-001 -->\n")

	stdout, _, err := runPRCmdInDir(t, repo, "pr-close", "42")
	if err != nil {
		t.Fatalf("pr-close: %v\nstdout: %s", err, stdout)
	}

	if _, err := os.Stat(filepath.Join(repo, ".squad", "items", "BUG-001-test.md")); !os.IsNotExist(err) {
		t.Fatalf("item should have moved out of items/, stat err = %v", err)
	}
	dest := filepath.Join(repo, ".squad", "done", "BUG-001-test.md")
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if !strings.Contains(string(b), "status: done") {
		t.Fatalf("frontmatter not updated; got:\n%s", b)
	}
}

func TestPRCloseNoMarkerIsNoop(t *testing.T) {
	repo := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())
	withFakeGH(t, "Plain PR description, no marker.")

	_, stderr, err := runPRCmdInDir(t, repo, "pr-close", "99")
	if err != nil {
		t.Fatalf("pr-close should exit 0, got: %v", err)
	}
	if !strings.Contains(stderr, "no squad-item marker") {
		t.Fatalf("expected skip message, got stderr: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(repo, ".squad", "items", "BUG-001-test.md")); err != nil {
		t.Fatalf("item must remain in items/: %v", err)
	}
}

func TestPRCloseIdempotent(t *testing.T) {
	repo := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())
	withFakeGH(t, "<!-- squad-item: BUG-001 -->")

	for i := 0; i < 2; i++ {
		_, _, err := runPRCmdInDir(t, repo, "pr-close", "42")
		if err != nil {
			t.Fatalf("run #%d: %v", i, err)
		}
	}
}

func TestPRCloseTwoMarkersFirstWins(t *testing.T) {
	repo := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())
	if err := os.WriteFile(
		filepath.Join(repo, ".squad", "items", "FEAT-002-other.md"),
		[]byte("---\nid: FEAT-002\nstatus: in-progress\n---\n"),
		0o644,
	); err != nil {
		t.Fatalf("write item 2: %v", err)
	}
	withFakeGH(t, "<!-- squad-item: BUG-001 -->\n<!-- squad-item: FEAT-002 -->")

	if _, _, err := runPRCmdInDir(t, repo, "pr-close", "42"); err != nil {
		t.Fatalf("pr-close: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".squad", "items", "FEAT-002-other.md")); err != nil {
		t.Fatalf("FEAT-002 must remain in items/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".squad", "done", "BUG-001-test.md")); err != nil {
		t.Fatalf("BUG-001 must be in done/: %v", err)
	}
}

func TestPRClosePersistsItemRowImmediately(t *testing.T) {
	repoDir := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())
	withFakeGH(t, "<!-- squad-item: BUG-001 -->")

	if _, _, err := runPRCmdInDir(t, repoDir, "pr-close", "42"); err != nil {
		t.Fatalf("pr-close: %v", err)
	}

	db, err := store.OpenDefault()
	if err != nil {
		t.Fatalf("open default db: %v", err)
	}
	defer db.Close()
	canonical, err := repo.Discover(repoDir)
	if err != nil {
		t.Fatalf("repo discover: %v", err)
	}
	repoID, err := repo.IDFor(canonical)
	if err != nil {
		t.Fatalf("repo id: %v", err)
	}
	var status string
	var archived int
	if err := db.QueryRow(
		`SELECT status, archived FROM items WHERE repo_id=? AND item_id=?`,
		repoID, "BUG-001",
	).Scan(&status, &archived); err != nil {
		t.Fatalf("items row missing after PRClose: %v", err)
	}
	if status != "done" {
		t.Errorf("status=%q want done", status)
	}
	if archived != 1 {
		t.Errorf("archived=%d want 1", archived)
	}
}

func TestPRCloseRespectsRepoIDFlag(t *testing.T) {
	repo := seedRepo(t)
	t.Setenv("SQUAD_HOME", t.TempDir())
	withFakeGH(t, "<!-- squad-item: BUG-001 -->")

	stdout, _, err := runPRCmdInDir(t, repo, "pr-close", "42", "--repo-id", "explicit-id-abc")
	if err != nil {
		t.Fatalf("pr-close: %v", err)
	}
	if !strings.Contains(stdout, "archived BUG-001") {
		t.Fatalf("expected archived line, got: %s", stdout)
	}
}
