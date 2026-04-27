package bootstrap

import (
	"context"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// Options bundles everything Ensure / Probe / Welcome need to act on the
// host. Manager is injected so tests can use a fake implementation.
type Options struct {
	BinaryPath string
	Bind       string
	Port       int
	HomeDir    string
	Manager    daemon.Manager
}

// Ensure brings the dashboard daemon to a known-good state for the current
// MCP connection: probes the running version, installs if absent, restarts
// if stale, reinstalls if the binary path drifted. Skeleton stub —
// install / restart / reinstall branches land in follow-ups.
func Ensure(ctx context.Context, opts Options) error {
	return nil
}
