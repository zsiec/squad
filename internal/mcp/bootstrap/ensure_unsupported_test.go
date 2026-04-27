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

func (errUnsupportedManager) Install(daemon.InstallOpts) error   { return daemon.ErrUnsupported }
func (errUnsupportedManager) Uninstall() error                   { return daemon.ErrUnsupported }
func (errUnsupportedManager) Status() (daemon.Status, error)     { return daemon.Status{}, daemon.ErrUnsupported }
func (errUnsupportedManager) Reinstall(daemon.InstallOpts) error { return daemon.ErrUnsupported }

func TestEnsure_UnsupportedPlatform_SetsBannerReturnsNil(t *testing.T) {
	_ = ConsumeBanner()

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
