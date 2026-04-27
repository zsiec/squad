package bootstrap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// upgradeDaemon is a self-contained fake daemon for the upgrade-flow
// tests. onRestart lets each test simulate launchd relaunching the binary
// at a new version (or, for the timeout case, decline to).
type upgradeDaemon struct {
	version    atomic.Value // string
	binaryPath atomic.Value // string
	pid        int
	startedAt  time.Time
	restartHit atomic.Int32
	onRestart  func(*upgradeDaemon)
}

func newUpgradeDaemon(version, binaryPath string) *upgradeDaemon {
	d := &upgradeDaemon{pid: 1234, startedAt: time.Now().UTC()}
	d.version.Store(version)
	d.binaryPath.Store(binaryPath)
	return d
}

func (d *upgradeDaemon) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		v, _ := d.version.Load().(string)
		bp, _ := d.binaryPath.Load().(string)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"` + v +
			`","binary_path":"` + bp +
			`","started_at":"` + d.startedAt.Format(time.RFC3339) +
			`","pid":` + itoa(d.pid) + `}`))
	})
	mux.HandleFunc("/api/_internal/restart", func(w http.ResponseWriter, r *http.Request) {
		d.restartHit.Add(1)
		w.WriteHeader(http.StatusAccepted)
		if d.onRestart != nil {
			d.onRestart(d)
		}
	})
	return mux
}

// TestUpgrade_VersionMismatch_RestartsAndPolls covers the headline
// scenario from the spec: probe reports daemon version A, configured
// version is B, restart endpoint flips the daemon to B, poll succeeds,
// upgrade banner is staged.
func TestUpgrade_VersionMismatch_RestartsAndPolls(t *testing.T) {
	shrinkTimings(t)
	_ = ConsumeBanner()

	d := newUpgradeDaemon("A", "/old/squad")
	d.onRestart = func(d *upgradeDaemon) { d.version.Store("B") }
	ts := httptest.NewServer(d.handler())
	defer ts.Close()
	setProbeBase(t, ts.URL)

	mgr := &recordingMgr{}
	if err := Ensure(context.Background(), Options{
		BinaryPath: "/old/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    t.TempDir(),
		Manager:    mgr,
		Version:    "B",
	}); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	if got := d.restartHit.Load(); got != 1 {
		t.Errorf("restart hit %d times, want 1", got)
	}
	if v, _ := d.version.Load().(string); v != "B" {
		t.Errorf("daemon still on version %q after restart, want B", v)
	}
	if mgr.installCalls.Load() != 0 || mgr.reinstallCalls.Load() != 0 {
		t.Errorf("Manager.Install/Reinstall must not run on pure version mismatch")
	}
	if got := ConsumeBanner(); !strings.Contains(got, "B") {
		t.Errorf("banner=%q, want upgrade copy mentioning version B", got)
	}
}

// TestUpgrade_BinaryPathDrift_Reinstalls covers `go install` to a new
// path: same advertised version, but the daemon is running the old
// binary on disk. Ensure must call Reinstall (which rewrites the plist
// / unit file) rather than just POST /api/_internal/restart.
func TestUpgrade_BinaryPathDrift_Reinstalls(t *testing.T) {
	shrinkTimings(t)
	_ = ConsumeBanner()

	d := newUpgradeDaemon("A", "/old/squad")
	ts := httptest.NewServer(d.handler())
	defer ts.Close()
	setProbeBase(t, ts.URL)

	mgr := &pathDriftMgr{recordingMgr: &recordingMgr{}, d: d}
	if err := Ensure(context.Background(), Options{
		BinaryPath: "/new/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    t.TempDir(),
		Manager:    mgr,
		Version:    "A",
	}); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	if got := mgr.reinstallCalls.Load(); got != 1 {
		t.Errorf("Reinstall calls=%d, want 1", got)
	}
	if got := mgr.installCalls.Load(); got != 0 {
		t.Errorf("Install must not run on pure path drift; calls=%d", got)
	}
	if got := d.restartHit.Load(); got != 0 {
		t.Errorf("restart endpoint hit %d times on path drift, want 0", got)
	}
}

// pathDriftMgr is the path-drift test's Manager: when Reinstall fires,
// it flips the live daemon's advertised binary path so the post-poll
// returns reachable+matched. Mirrors what launchd does in production.
type pathDriftMgr struct {
	*recordingMgr
	d *upgradeDaemon
}

func (m *pathDriftMgr) Reinstall(opts daemon.InstallOpts) error {
	if err := m.recordingMgr.Reinstall(opts); err != nil {
		return err
	}
	m.d.binaryPath.Store(opts.BinaryPath)
	return nil
}

// TestUpgrade_PollTimeout_ReturnsError covers the case where the
// daemon swallows the restart but never advertises the new version.
// Ensure must give up after pollDeadline rather than block the MCP
// boot indefinitely.
func TestUpgrade_PollTimeout_ReturnsError(t *testing.T) {
	prevD, prevC, prevT := pollDeadline, pollInterval, probeTimeout
	pollDeadline = 200 * time.Millisecond
	pollInterval = 20 * time.Millisecond
	probeTimeout = 50 * time.Millisecond
	t.Cleanup(func() { pollDeadline, pollInterval, probeTimeout = prevD, prevC, prevT })
	_ = ConsumeBanner()

	d := newUpgradeDaemon("A", "/old/squad")
	// onRestart deliberately omitted — version stays "A" forever.
	ts := httptest.NewServer(d.handler())
	defer ts.Close()
	setProbeBase(t, ts.URL)

	mgr := &recordingMgr{}
	err := Ensure(context.Background(), Options{
		BinaryPath: "/old/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    t.TempDir(),
		Manager:    mgr,
		Version:    "B",
	})
	if err == nil {
		t.Fatal("Ensure must return error when daemon never advertises new version")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error %q does not name the poll timeout", err)
	}
	// We must have actually attempted the restart even though the poll
	// gave up — otherwise the test would also pass for a regression
	// that simply never fires the restart.
	if got := d.restartHit.Load(); got != 1 {
		t.Errorf("restart endpoint hit %d times, want 1", got)
	}
	// Sanity: an unrelated wrapper error should not leak as the poll
	// timeout shape (e.g., a 401 from the restart endpoint).
	if errors.Is(err, http.ErrServerClosed) {
		t.Errorf("error wraps ErrServerClosed, want pure poll timeout")
	}
}
