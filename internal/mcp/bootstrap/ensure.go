package bootstrap

import (
	"context"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// Options bundles everything Ensure / Probe / Welcome need to act on the
// host. Manager and Opener are injected so tests can use fake
// implementations.
type Options struct {
	BinaryPath string
	Bind       string
	Port       int
	HomeDir    string
	Manager    daemon.Manager
	// Opener is invoked with the dashboard URL to launch a browser.
	// nil → defaultOpener (platform open / xdg-open).
	Opener func(url string) error
}

// Ensure brings the dashboard daemon to a known-good state for the current
// MCP connection: probes the running version, installs if absent, restarts
// if stale, reinstalls if the binary path drifted. Skeleton stub —
// install / restart / reinstall branches land in follow-ups.
func Ensure(ctx context.Context, opts Options) error {
	return nil
}
