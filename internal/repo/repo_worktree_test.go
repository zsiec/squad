package repo

import (
	"os"
	"path/filepath"
	"testing"
)

const fakeGitConfig = `[core]
	repositoryformatversion = 0
[remote "origin"]
	url = git@github.com:zsiec/squad.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`

// TestReadRemoteURL_RegularGitDir is the baseline: a regular checkout
// where `.git` is a directory containing the config. Pin the existing
// behavior so the worktree case below is a strictly additive change.
func TestReadRemoteURL_RegularGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(fakeGitConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRemoteURL(dir)
	if err != nil {
		t.Fatalf("ReadRemoteURL: %v", err)
	}
	if got != "git@github.com:zsiec/squad.git" {
		t.Errorf("ReadRemoteURL = %q; want git@github.com:zsiec/squad.git", got)
	}
}

// TestReadRemoteURL_WorktreeGitFile reproduces the BUG-045 shape: in a
// worktree, `<rootPath>/.git` is a regular file containing
// `gitdir: <path-to-shared-git-dir>/worktrees/<name>`, not a directory.
// The shared `config` lives in the main git dir, two levels up from
// the gitdir under the standard layout. ReadRemoteURL must follow the
// pointer and return the same URL the parent checkout would. Pre-fix,
// the function tried to `os.ReadFile(.git/config)` as if .git were a
// directory and returned `not a directory`.
func TestReadRemoteURL_WorktreeGitFile(t *testing.T) {
	tmp := t.TempDir()

	// Main checkout layout: <main>/.git/{config,worktrees/<name>/}
	mainGit := filepath.Join(tmp, "main", ".git")
	wtGitdir := filepath.Join(mainGit, "worktrees", "wt-fixture")
	if err := os.MkdirAll(wtGitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainGit, "config"), []byte(fakeGitConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	// Real git writes `commondir` containing the relative path back
	// to the main git dir. Honor that so the resolver can use it.
	if err := os.WriteFile(filepath.Join(wtGitdir, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Worktree layout: <wt>/.git is a file containing the pointer.
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+wtGitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadRemoteURL(wt)
	if err != nil {
		t.Fatalf("ReadRemoteURL on worktree: %v", err)
	}
	if got != "git@github.com:zsiec/squad.git" {
		t.Errorf("ReadRemoteURL = %q; want the parent checkout's origin URL", got)
	}
}

// TestReadRemoteURL_WorktreeGitFile_FallbackWithoutCommondir covers
// the legacy / hand-crafted case where the worktree gitdir does not
// have a `commondir` marker file. The fallback uses the standard
// layout assumption (gitdir lives at <main>/.git/worktrees/<name>),
// so two levels up from gitdir is the shared git dir.
func TestReadRemoteURL_WorktreeGitFile_FallbackWithoutCommondir(t *testing.T) {
	tmp := t.TempDir()
	mainGit := filepath.Join(tmp, "main", ".git")
	wtGitdir := filepath.Join(mainGit, "worktrees", "no-commondir")
	if err := os.MkdirAll(wtGitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainGit, "config"), []byte(fakeGitConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+wtGitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadRemoteURL(wt)
	if err != nil {
		t.Fatalf("ReadRemoteURL on worktree without commondir: %v", err)
	}
	if got != "git@github.com:zsiec/squad.git" {
		t.Errorf("ReadRemoteURL = %q; want the parent checkout's origin URL", got)
	}
}

// TestReadRemoteURL_NoGitReturnsEmpty pins the existing graceful
// behavior when neither a .git directory nor file is present. Squad
// runs in non-git dirs (CI, fresh-init scaffolding) and must not
// error there.
func TestReadRemoteURL_NoGitReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	got, err := ReadRemoteURL(dir)
	if err != nil {
		t.Fatalf("ReadRemoteURL on plain dir: %v", err)
	}
	if got != "" {
		t.Errorf("ReadRemoteURL on plain dir = %q; want empty", got)
	}
}
