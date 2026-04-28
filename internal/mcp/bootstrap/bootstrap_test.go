package bootstrap

import (
	"context"
	"testing"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// fakeManager satisfies daemon.Manager for injection tests. The skeleton
// only proves the seam compiles; later items exercise the methods.
type fakeManager struct{}

func (fakeManager) Install(daemon.InstallOpts) error   { return nil }
func (fakeManager) Uninstall() error                   { return nil }
func (fakeManager) Status() (daemon.Status, error)     { return daemon.Status{}, nil }
func (fakeManager) Reinstall(daemon.InstallOpts) error { return nil }

func TestSeamsCallable(t *testing.T) {
	t.Setenv("SQUAD_NO_AUTO_DAEMON", "1")
	ctx := context.Background()
	opts := Options{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    t.TempDir(),
		Manager:    fakeManager{},
	}

	if err := Ensure(ctx, opts); err != nil {
		t.Fatalf("Ensure returned %v, want nil under SQUAD_NO_AUTO_DAEMON=1", err)
	}

	if _, err := Probe(ctx); err != nil {
		t.Fatalf("Probe returned %v, want nil", err)
	}

	if err := Welcome(ctx, opts); err != nil {
		t.Fatalf("Welcome returned %v, want nil", err)
	}
}
