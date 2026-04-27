package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// pollDeadline + pollInterval bound the wait Ensure does after install /
// restart / reinstall: enough for a cold launchd bootstrap on a busy
// laptop, short enough that a stuck daemon doesn't strand MCP boot. The
// AC explicitly does not expose these as user options.
var (
	pollDeadline = 10 * time.Second
	pollInterval = 200 * time.Millisecond
)

// Options bundles everything Ensure / Probe / Welcome need to act on the
// host. Manager and Opener are injected so tests can use fake
// implementations.
type Options struct {
	BinaryPath string
	Bind       string
	Port       int
	HomeDir    string
	// SquadHome is the canonical squad-home (typically store.Home() — honors
	// SQUAD_HOME, defaults to $HOME/.squad). When set, Welcome uses it for
	// the .welcomed sentinel path. When empty, Welcome falls back to
	// filepath.Join(HomeDir, ".squad") for back-compat with callers that
	// haven't been updated. Production should always set it from store.Home()
	// so the sentinel check survives SQUAD_HOME drift.
	SquadHome string
	Manager   daemon.Manager
	Version   string
	// Opener is invoked with the dashboard URL to launch a browser.
	// nil → defaultOpener (platform open / xdg-open).
	Opener func(url string) error
}

// Ensure brings the dashboard daemon to a known-good state for the
// current MCP connection: probes the running version, installs if
// absent, restarts if stale, reinstalls if the binary path drifted.
// SQUAD_NO_AUTO_DAEMON=1 short-circuits to nil. ErrUnsupported is a
// graceful skip — Ensure logs, stages BannerUnsupported, and returns
// nil so MCP keeps serving tools without the UI. Any other failure is
// logged and returned wrapped.
func Ensure(ctx context.Context, opts Options) error {
	if os.Getenv("SQUAD_NO_AUTO_DAEMON") == "1" {
		return nil
	}
	probe, err := Probe(ctx)
	if err != nil {
		return logAndWrap("probe daemon", err)
	}
	switch {
	case !probe.Present:
		return ensureInstall(ctx, opts)
	case probe.Version != opts.Version:
		return ensureRestart(ctx, opts)
	case probe.BinaryPath != opts.BinaryPath:
		return ensureReinstall(ctx, opts)
	default:
		return nil
	}
}

func ensureInstall(ctx context.Context, opts Options) error {
	lockPath := filepath.Join(opts.HomeDir, ".squad", "install.lock")
	release, err := acquireInstallLock(lockPath)
	if err != nil {
		return logAndWrap("acquire install lock", err)
	}
	defer release()
	// Re-probe under the lock: a parallel MCP session may have completed
	// the install while we waited.
	if probe, err := Probe(ctx); err == nil && probe.Present {
		return nil
	}
	installOpts := daemon.InstallOpts{
		BinaryPath: opts.BinaryPath,
		Bind:       opts.Bind,
		Port:       opts.Port,
		LogDir:     filepath.Join(opts.HomeDir, ".squad", "logs"),
		HomeDir:    opts.HomeDir,
	}
	if err := opts.Manager.Install(installOpts); err != nil {
		if handleUnsupported(err) {
			return nil
		}
		return logAndWrap("install daemon", err)
	}
	if err := waitUntilPresent(ctx); err != nil {
		return logAndWrap("wait for daemon", err)
	}
	SetBanner(BannerInstalled(opts.Port))
	return nil
}

func ensureRestart(ctx context.Context, opts Options) error {
	reqCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, probeBase+"/api/_internal/restart", nil)
	if err != nil {
		return logAndWrap("build restart request", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return logAndWrap("post restart", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return logAndWrap("post restart", fmt.Errorf("status %d", resp.StatusCode))
	}
	if err := waitUntilVersion(ctx, opts.Version); err != nil {
		return logAndWrap("wait for upgraded daemon", err)
	}
	SetBanner(BannerUpgraded(opts.Version))
	return nil
}

func ensureReinstall(ctx context.Context, opts Options) error {
	installOpts := daemon.InstallOpts{
		BinaryPath: opts.BinaryPath,
		Bind:       opts.Bind,
		Port:       opts.Port,
		LogDir:     filepath.Join(opts.HomeDir, ".squad", "logs"),
		HomeDir:    opts.HomeDir,
	}
	if err := opts.Manager.Reinstall(installOpts); err != nil {
		if handleUnsupported(err) {
			return nil
		}
		return logAndWrap("reinstall daemon", err)
	}
	if err := waitUntilPresent(ctx); err != nil {
		return logAndWrap("wait for reinstalled daemon", err)
	}
	SetBanner(BannerUpgraded(opts.Version))
	return nil
}

// handleUnsupported returns true if err is the daemon's
// "this platform is not supported" sentinel. Side-effects: log a one-line
// hint to stderr and stage the unsupported banner. Callers treat the true
// return as "stop, don't retry, surface the banner on the next tool call".
func handleUnsupported(err error) bool {
	if !errors.Is(err, daemon.ErrUnsupported) {
		return false
	}
	fmt.Fprintln(os.Stderr, `squad: dashboard auto-install not supported on this platform; run "squad serve" manually for the UI`)
	SetBanner(BannerUnsupported)
	return true
}

func waitUntilPresent(ctx context.Context) error {
	return pollUntil(ctx, func(ctx context.Context) (bool, error) {
		p, err := Probe(ctx)
		return p.Present, err
	})
}

func waitUntilVersion(ctx context.Context, want string) error {
	return pollUntil(ctx, func(ctx context.Context) (bool, error) {
		p, err := Probe(ctx)
		return p.Present && p.Version == want, err
	})
}

func pollUntil(ctx context.Context, check func(context.Context) (bool, error)) error {
	deadline := time.NewTimer(pollDeadline)
	defer deadline.Stop()
	for {
		ok, err := check(ctx)
		if err == nil && ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("timed out")
		case <-time.After(pollInterval):
		}
	}
}

func logAndWrap(stage string, err error) error {
	wrapped := fmt.Errorf("%s: %w", stage, err)
	fmt.Fprintf(os.Stderr, "squad: bootstrap %s\n", wrapped.Error())
	return wrapped
}
