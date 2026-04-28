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

type recordingMgr struct {
	installCalls   atomic.Int32
	uninstallCalls atomic.Int32
	reinstallCalls atomic.Int32
	lastOpts       daemon.InstallOpts
	installErr     error
	reinstallErr   error
}

func (m *recordingMgr) Install(opts daemon.InstallOpts) error {
	m.installCalls.Add(1)
	m.lastOpts = opts
	return m.installErr
}
func (m *recordingMgr) Uninstall() error               { m.uninstallCalls.Add(1); return nil }
func (m *recordingMgr) Status() (daemon.Status, error) { return daemon.Status{}, nil }
func (m *recordingMgr) Reinstall(opts daemon.InstallOpts) error {
	m.reinstallCalls.Add(1)
	m.lastOpts = opts
	return m.reinstallErr
}

// shrinkTimings tightens Ensure's poll deadline + cadence for tests so
// failures surface in <1s instead of 10s.
func shrinkTimings(t *testing.T) {
	t.Helper()
	prevD, prevC, prevT := pollDeadline, pollInterval, probeTimeout
	pollDeadline = 500 * time.Millisecond
	pollInterval = 20 * time.Millisecond
	probeTimeout = 100 * time.Millisecond
	t.Cleanup(func() { pollDeadline, pollInterval, probeTimeout = prevD, prevC, prevT })
}

func newEnsureOpts(t *testing.T, mgr daemon.Manager) Options {
	t.Helper()
	return Options{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		HomeDir:    t.TempDir(),
		Manager:    mgr,
		Version:    "1.2.3",
	}
}

// fakeDaemon is an httptest server that pretends to be the running
// daemon; its version + binary_path are mutated atomically across calls
// so tests can simulate restart / reinstall completing.
type fakeDaemon struct {
	version    atomic.Value // string
	binaryPath atomic.Value // string
	pid        int
	started    time.Time
	restartHit atomic.Int32
}

func newFakeDaemon(version, binaryPath string) *fakeDaemon {
	d := &fakeDaemon{pid: 4321, started: time.Now().UTC()}
	d.version.Store(version)
	d.binaryPath.Store(binaryPath)
	return d
}

func (d *fakeDaemon) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		v, _ := d.version.Load().(string)
		bp, _ := d.binaryPath.Load().(string)
		_, _ = w.Write([]byte(`{"version":"` + v + `","binary_path":"` + bp + `","started_at":"` + d.started.Format(time.RFC3339) + `","pid":` + itoa(d.pid) + `}`))
	})
	mux.HandleFunc("/api/_internal/restart", func(w http.ResponseWriter, r *http.Request) {
		d.restartHit.Add(1)
		w.WriteHeader(http.StatusAccepted)
	})
	return mux
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestEnsure_NoAutoDaemonEnv_ShortCircuits(t *testing.T) {
	t.Setenv("SQUAD_NO_AUTO_DAEMON", "1")
	mgr := &recordingMgr{}
	if err := Ensure(context.Background(), newEnsureOpts(t, mgr)); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got := mgr.installCalls.Load(); got != 0 {
		t.Errorf("Install invoked %d times under SQUAD_NO_AUTO_DAEMON=1", got)
	}
}

func TestEnsure_DaemonAbsent_InstallsAndPolls(t *testing.T) {
	shrinkTimings(t)
	d := newFakeDaemon("1.2.3", "/usr/local/bin/squad")
	ts := httptest.NewUnstartedServer(d.handler())
	t.Cleanup(ts.Close)
	setProbeBase(t, "http://127.0.0.1:1") // refused: daemon absent

	mgr := &installFlipper{
		recordingMgr: &recordingMgr{},
		ts:           ts,
		t:            t,
	}
	opts := newEnsureOpts(t, mgr)
	if err := Ensure(context.Background(), opts); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got := mgr.installCalls.Load(); got != 1 {
		t.Errorf("Install calls=%d want 1", got)
	}
}

// installFlipper starts the embedded test server when Install is called
// and redirects probeBase to its URL, so the post-install poll finds
// the daemon up. Mirrors what launchd / systemd-user do in production.
type installFlipper struct {
	*recordingMgr
	ts *httptest.Server
	t  *testing.T
}

func (i *installFlipper) Install(opts daemon.InstallOpts) error {
	if err := i.recordingMgr.Install(opts); err != nil {
		return err
	}
	i.ts.Start()
	prev := probeBase
	probeBase = i.ts.URL
	i.t.Cleanup(func() { probeBase = prev })
	return nil
}

func TestEnsure_VersionMismatch_PostsRestartAndPolls(t *testing.T) {
	shrinkTimings(t)
	d := newFakeDaemon("0.9.0", "/usr/local/bin/squad")
	ts := httptest.NewServer(d.handler())
	defer ts.Close()
	setProbeBase(t, ts.URL)

	mgr := &recordingMgr{}
	opts := newEnsureOpts(t, mgr)
	opts.Version = "1.0.0" // newer than daemon's 0.9.0

	// Background goroutine flips the daemon to the new version after the
	// first /api/_internal/restart hit, simulating launchd relaunching it.
	go func() {
		for d.restartHit.Load() == 0 {
			time.Sleep(5 * time.Millisecond)
		}
		d.version.Store("1.0.0")
	}()

	if err := Ensure(context.Background(), opts); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got := d.restartHit.Load(); got != 1 {
		t.Errorf("restart endpoint hits=%d want 1", got)
	}
	if mgr.installCalls.Load() != 0 || mgr.reinstallCalls.Load() != 0 {
		t.Errorf("expected no install/reinstall on version mismatch")
	}
}

func TestEnsure_BinaryPathMismatch_Reinstalls(t *testing.T) {
	shrinkTimings(t)
	d := newFakeDaemon("1.0.0", "/old/path/squad")
	ts := httptest.NewServer(d.handler())
	defer ts.Close()
	setProbeBase(t, ts.URL)

	mgr := &recordingMgr{}
	opts := newEnsureOpts(t, mgr)
	opts.Version = "1.0.0"
	opts.BinaryPath = "/new/path/squad"

	mgrSwitch := &reinstallSwap{recordingMgr: mgr, d: d}
	opts.Manager = mgrSwitch

	if err := Ensure(context.Background(), opts); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if got := mgrSwitch.reinstallCalls.Load(); got != 1 {
		t.Errorf("Reinstall calls=%d want 1", got)
	}
	if mgrSwitch.installCalls.Load() != 0 {
		t.Errorf("Install should not be called on path mismatch")
	}
}

type reinstallSwap struct {
	*recordingMgr
	d *fakeDaemon
}

func (r *reinstallSwap) Reinstall(opts daemon.InstallOpts) error {
	_ = r.recordingMgr.Reinstall(opts)
	r.d.binaryPath.Store(opts.BinaryPath)
	return nil
}

func TestEnsure_VersionAndPathMatch_NoOp(t *testing.T) {
	shrinkTimings(t)
	d := newFakeDaemon("1.2.3", "/usr/local/bin/squad")
	ts := httptest.NewServer(d.handler())
	defer ts.Close()
	setProbeBase(t, ts.URL)

	mgr := &recordingMgr{}
	if err := Ensure(context.Background(), newEnsureOpts(t, mgr)); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if mgr.installCalls.Load() != 0 || mgr.reinstallCalls.Load() != 0 {
		t.Error("Ensure must be a no-op when daemon is current")
	}
	if got := d.restartHit.Load(); got != 0 {
		t.Errorf("restart hit %d times on noop, want 0", got)
	}
}

func TestEnsure_InstallErrorWrappedAndLogged(t *testing.T) {
	shrinkTimings(t)
	setProbeBase(t, "http://127.0.0.1:1") // refused → daemon absent
	mgr := &recordingMgr{installErr: errors.New("plist denied")}
	err := Ensure(context.Background(), newEnsureOpts(t, mgr))
	if err == nil {
		t.Fatal("Ensure returned nil; want wrapped install error")
	}
	if !strings.Contains(err.Error(), "plist denied") {
		t.Errorf("error %q does not wrap install cause", err)
	}
}
