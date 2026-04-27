package bootstrap

import (
	"context"
	"testing"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// errUnsupportedManager is a daemon.Manager whose Install / Reinstall
// return ErrUnsupported — the shape of the platform-stub Manager from
// internal/tui/daemon/install_other.go on Windows / unsupported OSes.
type errUnsupportedManager struct{}

func (errUnsupportedManager) Install(daemon.InstallOpts) error { return daemon.ErrUnsupported }
func (errUnsupportedManager) Uninstall() error                 { return daemon.ErrUnsupported }
func (errUnsupportedManager) Status() (daemon.Status, error) {
	return daemon.Status{}, daemon.ErrUnsupported
}
func (errUnsupportedManager) Reinstall(daemon.InstallOpts) error { return daemon.ErrUnsupported }

func TestEnsure_UnsupportedPlatform_SetsBannerReturnsNil(t *testing.T) {
	_ = ConsumeBanner()
	// Pin probeBase to a refused port so a developer running a real
	// dashboard on 127.0.0.1:7777 doesn't push this test through the
	// restart branch (which would error on the unreachable POST) instead
	// of the install branch where ErrUnsupported lives.
	setProbeBase(t, "http://127.0.0.1:1")

	opts := Options{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    t.TempDir(),
		Manager:    errUnsupportedManager{},
	}

	if err := Ensure(context.Background(), opts); err != nil {
		t.Fatalf("Ensure should return nil on ErrUnsupported, got %v", err)
	}

	if got := ConsumeBanner(); got != BannerUnsupported {
		t.Errorf("banner = %q, want %q", got, BannerUnsupported)
	}
}
