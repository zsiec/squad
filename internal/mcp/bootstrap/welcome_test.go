package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func newWelcomeOpts(home string, opener func(string) error) Options {
	return Options{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    home,
		Manager:    fakeManager{},
		Opener:     opener,
	}
}

func TestWelcome_AbsentSentinel_OpensAndWritesSentinel(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "")

	var calls atomic.Int32
	var gotURL string
	opener := func(url string) error {
		calls.Add(1)
		gotURL = url
		return nil
	}

	if err := Welcome(context.Background(), newWelcomeOpts(tmp, opener)); err != nil {
		t.Fatalf("Welcome: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("opener invoked %d times, want 1", got)
	}
	if gotURL != "http://localhost:7777" {
		t.Errorf("opener URL = %q, want http://localhost:7777", gotURL)
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
	t.Setenv("SQUAD_NO_BROWSER", "")
	sentinel := filepath.Join(tmp, ".squad", ".welcomed")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	opener := func(url string) error {
		calls.Add(1)
		return nil
	}

	if err := Welcome(context.Background(), newWelcomeOpts(tmp, opener)); err != nil {
		t.Fatalf("Welcome: %v", err)
	}

	if got := calls.Load(); got != 0 {
		t.Errorf("opener invoked %d times on present sentinel, want 0", got)
	}
}

func TestWelcome_NoBrowserEnv_SkipsOpenWritesSentinel(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "1")

	var calls atomic.Int32
	opener := func(url string) error {
		calls.Add(1)
		return nil
	}

	if err := Welcome(context.Background(), newWelcomeOpts(tmp, opener)); err != nil {
		t.Fatalf("Welcome: %v", err)
	}

	if got := calls.Load(); got != 0 {
		t.Errorf("opener invoked %d times under SQUAD_NO_BROWSER=1, want 0", got)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".squad", ".welcomed")); err != nil {
		t.Errorf("sentinel should be written even under SQUAD_NO_BROWSER=1: %v", err)
	}
}

func TestWelcome_OpenerFails_StillWritesSentinelNoError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "")

	opener := func(url string) error { return errors.New("boom") }

	if err := Welcome(context.Background(), newWelcomeOpts(tmp, opener)); err != nil {
		t.Fatalf("Welcome should not return error when opener fails: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, ".squad", ".welcomed")); err != nil {
		t.Errorf("sentinel should still be written after opener failure: %v", err)
	}
}

// TestWelcome_NonDirHomeDir_DoesNotOpen covers the case where HomeDir is
// not a directory (a file in its place). os.Stat on the would-be sentinel
// path returns ENOTDIR, which the new contract treats like any other
// non-ENOENT error: log to stderr and skip the opener. The bias toward
// not-spamming applies uniformly across sentinel-stat failure modes.
func TestWelcome_NonDirHomeDir_DoesNotOpen(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "")

	bogus := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(bogus, []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	opener := func(url string) error { calls.Add(1); return nil }
	opts := newWelcomeOpts(bogus, opener)

	if err := Welcome(context.Background(), opts); err != nil {
		t.Fatalf("Welcome should not error on stat-ENOTDIR (it should skip): %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("opener invoked %d times; bias must be toward not-opening", got)
	}
}

// TestWelcome_UsesSquadHomeOverHomeDirSubpath pins the HomeDir-drift fix.
// When SquadHome is supplied, the sentinel path must come from there — not
// from filepath.Join(HomeDir, ".squad"). This protects the SQUAD_HOME-set
// case, where the canonical squad home does not match $HOME/.squad and
// the sentinel written by store.Home()-aware code lives under SquadHome.
func TestWelcome_UsesSquadHomeOverHomeDirSubpath(t *testing.T) {
	homeDir := t.TempDir()
	squadHome := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "")

	// Sentinel under the canonical squad-home (not HomeDir/.squad).
	if err := os.WriteFile(filepath.Join(squadHome, ".welcomed"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	opener := func(url string) error { calls.Add(1); return nil }

	opts := newWelcomeOpts(homeDir, opener)
	opts.SquadHome = squadHome

	if err := Welcome(context.Background(), opts); err != nil {
		t.Fatalf("Welcome: %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("opener invoked %d times despite sentinel under SquadHome, want 0", got)
	}
}

// TestWelcome_StatNonENOENTError_DoesNotOpen pins the bias-toward-not-
// spamming fix. When os.Stat returns an error that is not ENOENT (e.g.
// EACCES on a chmod 000 .squad dir, or a transient FS error), Welcome
// must skip the opener instead of falling through to the write+open
// path. Better to silently miss a first-run for one boot than to spam
// the user's browser on every boot under a flaky filesystem.
func TestWelcome_StatNonENOENTError_DoesNotOpen(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "")

	squadDir := filepath.Join(tmp, ".squad")
	if err := os.MkdirAll(squadDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(squadDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(squadDir, 0o755) })

	var calls atomic.Int32
	opener := func(url string) error { calls.Add(1); return nil }

	if err := Welcome(context.Background(), newWelcomeOpts(tmp, opener)); err != nil {
		t.Fatalf("Welcome should not error on stat-EACCES (it should skip): %v", err)
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("opener invoked %d times on stat-EACCES; bias must be toward not-opening", got)
	}
}

func TestWelcome_AtomicWrite_NoLingeringTempFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "1")
	opener := func(url string) error { return nil }

	if err := Welcome(context.Background(), newWelcomeOpts(tmp, opener)); err != nil {
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
