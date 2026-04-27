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

func TestWelcome_SentinelWriteFails_NoError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "1")

	// HomeDir does not exist as a directory — make it a file so MkdirAll/WriteFile fails.
	bogus := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(bogus, []byte("blocker"), 0o644); err != nil {
		t.Fatal(err)
	}

	opener := func(url string) error { return nil }
	opts := newWelcomeOpts(bogus, opener)

	if err := Welcome(context.Background(), opts); err != nil {
		t.Fatalf("Welcome should not return error on sentinel write failure: %v", err)
	}
}
