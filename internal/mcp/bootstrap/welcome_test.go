package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func newWelcomeOpts(home string) Options {
	return Options{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    home,
		Manager:    fakeManager{},
	}
}

func TestWelcome_AbsentSentinel_WritesSentinel(t *testing.T) {
	tmp := t.TempDir()

	if err := Welcome(context.Background(), newWelcomeOpts(tmp)); err != nil {
		t.Fatalf("Welcome: %v", err)
	}

	sentinel := filepath.Join(tmp, ".squad", ".welcomed")
	info, err := os.Stat(sentinel)
	if err != nil {
		t.Fatalf("sentinel should exist: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("sentinel should be zero-byte, got %d", info.Size())
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Errorf("sentinel mode = %o, want 0644", mode)
	}
}

func TestWelcome_PresentSentinel_NoOp(t *testing.T) {
	tmp := t.TempDir()
	sentinel := filepath.Join(tmp, ".squad", ".welcomed")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pre, err := os.Stat(sentinel)
	if err != nil {
		t.Fatal(err)
	}

	if err := Welcome(context.Background(), newWelcomeOpts(tmp)); err != nil {
		t.Fatalf("Welcome: %v", err)
	}

	post, err := os.Stat(sentinel)
	if err != nil {
		t.Fatalf("sentinel should still exist: %v", err)
	}
	if !post.ModTime().Equal(pre.ModTime()) {
		t.Errorf("sentinel mtime changed (%v -> %v); Welcome must no-op when sentinel present",
			pre.ModTime(), post.ModTime())
	}
}

// TestWelcome_NonDirHomeDir_BiasesToSilence covers the case where HomeDir
// is not a directory (a file in its place). os.Stat on the would-be
// sentinel path returns ENOTDIR, which the contract treats like any other
// non-ENOENT error: log to stderr and skip. Bias toward silence on
// flaky filesystem shapes.
func TestWelcome_NonDirHomeDir_BiasesToSilence(t *testing.T) {
	tmp := t.TempDir()

	bogus := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(bogus, []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Welcome(context.Background(), newWelcomeOpts(bogus)); err != nil {
		t.Fatalf("Welcome should not error on stat-ENOTDIR (it should skip): %v", err)
	}
}

// TestWelcome_UsesSquadHomeOverHomeDirSubpath pins the HomeDir-drift fix.
// When SquadHome is supplied, the sentinel path comes from there — not
// from filepath.Join(HomeDir, ".squad").
func TestWelcome_UsesSquadHomeOverHomeDirSubpath(t *testing.T) {
	homeDir := t.TempDir()
	squadHome := t.TempDir()

	// Sentinel under the canonical squad-home (not HomeDir/.squad).
	if err := os.WriteFile(filepath.Join(squadHome, ".welcomed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	opts := newWelcomeOpts(homeDir)
	opts.SquadHome = squadHome

	if err := Welcome(context.Background(), opts); err != nil {
		t.Fatalf("Welcome: %v", err)
	}

	// A no-op call must NOT have written a sentinel under HomeDir/.squad.
	if _, err := os.Stat(filepath.Join(homeDir, ".squad", ".welcomed")); !os.IsNotExist(err) {
		t.Errorf("Welcome wrote a HomeDir/.squad sentinel despite the SquadHome one existing: err=%v", err)
	}
}

// TestWelcome_StatNonENOENTError_BiasesToSilence pins the bias-toward-
// silence policy: a stat error other than ENOENT (e.g. EACCES on a
// chmod 000 .squad dir) skips the write rather than blindly retrying.
// Better to silently miss a first-run write than to thrash on a flaky
// filesystem.
func TestWelcome_StatNonENOENTError_BiasesToSilence(t *testing.T) {
	tmp := t.TempDir()

	squadDir := filepath.Join(tmp, ".squad")
	if err := os.MkdirAll(squadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(squadDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(squadDir, 0o755) })

	if err := Welcome(context.Background(), newWelcomeOpts(tmp)); err != nil {
		t.Fatalf("Welcome should not error on stat-EACCES (it should skip): %v", err)
	}
}

func TestWelcome_AtomicWrite_NoLingeringTempFile(t *testing.T) {
	tmp := t.TempDir()
	if err := Welcome(context.Background(), newWelcomeOpts(tmp)); err != nil {
		t.Fatalf("Welcome: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(tmp, ".squad"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == ".welcomed" {
			continue
		}
		t.Errorf("unexpected leftover file in .squad/: %s (atomic write should not leak temp files)", e.Name())
	}
}
