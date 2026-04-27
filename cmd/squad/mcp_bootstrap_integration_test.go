package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zsiec/squad/internal/mcp/bootstrap"
	"github.com/zsiec/squad/internal/tui/daemon"
)

// bootstrapRecordingMgr is a daemon.Manager fake that counts Install / Reinstall /
// Uninstall calls and never touches a real plist or systemd unit. The
// integration test asserts the install path (clean home) actually fires
// through the real bootstrap.Ensure logic, not just through a hook.
type bootstrapRecordingMgr struct {
	installCalls   atomic.Int32
	reinstallCalls atomic.Int32
}

func (m *bootstrapRecordingMgr) Install(daemon.InstallOpts) error { m.installCalls.Add(1); return nil }
func (m *bootstrapRecordingMgr) Uninstall() error                 { return nil }
func (m *bootstrapRecordingMgr) Status() (daemon.Status, error)   { return daemon.Status{}, nil }
func (m *bootstrapRecordingMgr) Reinstall(daemon.InstallOpts) error {
	m.reinstallCalls.Add(1)
	return nil
}

// failingMgr returns the configured error from Install / Reinstall so
// the failure-path test can assert MCP keeps serving even when
// bootstrap can't bring up the dashboard.
type failingMgr struct {
	bootstrapRecordingMgr
	err error
}

func (m *failingMgr) Install(opts daemon.InstallOpts) error {
	m.installCalls.Add(1)
	return m.err
}
func (m *failingMgr) Reinstall(opts daemon.InstallOpts) error {
	m.reinstallCalls.Add(1)
	return m.err
}

// versionResponder returns an httptest.HandlerFunc that always
// advertises the same daemon version + binary path (i.e. a daemon that
// is up and matched). Tests reuse this for the rerun + opt-out paths.
func versionResponder(version, binaryPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     version,
			"binary_path": binaryPath,
			"started_at":  "2026-04-27T00:00:00Z",
			"pid":         12345,
		})
	}
}

// TestMCP_RealBootstrap_InstallPathHitOnCleanHome wires the real
// bootstrap.Ensure path (Probe → daemon absent → Install) through
// runMCP and asserts the daemon.Manager.Install was actually called and
// the post-install banner reaches the first tools/call response. Pairs
// with the hook-based wiring tests; this one proves the full
// integration composes end-to-end.
func TestMCP_RealBootstrap_InstallPathHitOnCleanHome(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { _ = bootstrap.ConsumeBanner() })

	// httptest.Server stands in for the daemon. /api/version returns 404
	// the first time (so Ensure's Probe sees Present=false → install
	// branch); subsequent polls return 200 with the version and binary
	// path the install was supposed to produce, so Ensure's
	// waitUntilPresent succeeds without us actually running launchd.
	bin := "/test/bin/squad"
	const ver = "test-version"
	var probeCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/version" {
			http.NotFound(w, r)
			return
		}
		// Calls 1 + 2 → 404 (initial Probe sees clean home; ensureInstall's
		// re-probe-under-lock confirms still absent so the Install branch
		// fires). Calls 3+ → 200 (post-install waitUntilPresent poll).
		if probeCalls.Add(1) <= 2 {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     ver,
			"binary_path": bin,
			"started_at":  "2026-04-27T00:00:00Z",
			"pid":         12345,
		})
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(bootstrap.SetProbeBaseForTest(srv.URL))

	mgr := &bootstrapRecordingMgr{}
	homeDir := t.TempDir()
	t.Setenv("SQUAD_NO_BROWSER", "1")
	hook := func(ctx context.Context) {
		opts := bootstrap.Options{
			BinaryPath: bin,
			Bind:       "127.0.0.1",
			Port:       7777,
			HomeDir:    homeDir,
			Manager:    mgr,
			Version:    ver,
		}
		if err := bootstrap.Ensure(ctx, opts); err != nil {
			t.Logf("Ensure: %v", err)
			return
		}
		_ = bootstrap.Welcome(ctx, opts)
	}

	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_whoami","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out, WithBootstrap(hook)); err != nil {
		t.Fatalf("runMCP: %v", err)
	}

	if got := mgr.installCalls.Load(); got != 1 {
		t.Errorf("Manager.Install calls = %d, want 1 (clean home should trigger install path)", got)
	}
	if got := mgr.reinstallCalls.Load(); got != 0 {
		t.Errorf("Manager.Reinstall calls = %d, want 0", got)
	}

	// Welcome should have written the .welcomed sentinel under HomeDir/.squad.
	if _, err := os.Stat(homeDir + "/.squad/.welcomed"); err != nil {
		t.Errorf("welcome sentinel missing: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d:\n%s", len(lines), out.String())
	}
	var resp struct {
		Result struct {
			Content []map[string]any `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	wantBanner := bootstrap.BannerInstalled(7777)
	if len(resp.Result.Content) < 2 || resp.Result.Content[0]["text"] != wantBanner {
		t.Errorf("first content block should be install banner %q, got content=%v", wantBanner, resp.Result.Content)
	}
}

// TestMCP_RealBootstrap_RerunPath_NoOp covers the steady-state case:
// daemon already up at the expected version + binary path, .welcomed
// sentinel already on disk. Bootstrap must do nothing — no Install, no
// opener, no banner — and the first tools/call response must NOT carry
// a banner.
func TestMCP_RealBootstrap_RerunPath_NoOp(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { _ = bootstrap.ConsumeBanner() })
	_ = bootstrap.ConsumeBanner()

	bin := "/test/bin/squad"
	const ver = "test-version"
	srv := httptest.NewServer(versionResponder(ver, bin))
	t.Cleanup(srv.Close)
	t.Cleanup(bootstrap.SetProbeBaseForTest(srv.URL))

	homeDir := t.TempDir()
	if err := os.MkdirAll(homeDir+"/.squad", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homeDir+"/.squad/.welcomed", nil, 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := &bootstrapRecordingMgr{}
	var openerCalls atomic.Int32
	hook := func(ctx context.Context) {
		opts := bootstrap.Options{
			BinaryPath: bin,
			Bind:       "127.0.0.1",
			Port:       7777,
			HomeDir:    homeDir,
			Manager:    mgr,
			Version:    ver,
			Opener:     func(string) error { openerCalls.Add(1); return nil },
		}
		_ = bootstrap.Ensure(ctx, opts)
		_ = bootstrap.Welcome(ctx, opts)
	}

	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_whoami","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out, WithBootstrap(hook)); err != nil {
		t.Fatalf("runMCP: %v", err)
	}

	if got := mgr.installCalls.Load(); got != 0 {
		t.Errorf("Install called %d times on rerun, want 0", got)
	}
	if got := mgr.reinstallCalls.Load(); got != 0 {
		t.Errorf("Reinstall called %d times on rerun, want 0", got)
	}
	if got := openerCalls.Load(); got != 0 {
		t.Errorf("opener called %d times when sentinel present, want 0", got)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d:\n%s", len(lines), out.String())
	}
	var resp struct {
		Result struct {
			Content []map[string]any `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp); err != nil {
		t.Fatalf("decode tools/call: %v", err)
	}
	// A quiet rerun must produce exactly one content block (the whoami
	// JSON). server.go prepends a banner block iff ConsumeBanner is
	// non-empty; a length check is the tighter invariant than scanning
	// for known banner prefixes.
	if got := len(resp.Result.Content); got != 1 {
		t.Errorf("rerun should produce 1 content block, got %d: %v", got, resp.Result.Content)
	}
}

// TestMCP_RealBootstrap_NoAutoDaemonEnv_ShortCircuits asserts that
// SQUAD_NO_AUTO_DAEMON=1 prevents Manager.Install from running even on
// a clean home with no daemon reachable. MCP must still serve tools.
func TestMCP_RealBootstrap_NoAutoDaemonEnv_ShortCircuits(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { _ = bootstrap.ConsumeBanner() })
	_ = bootstrap.ConsumeBanner()

	t.Setenv("SQUAD_NO_AUTO_DAEMON", "1")
	t.Setenv("SQUAD_NO_BROWSER", "1")
	// No probe server pointed at — the env var must short-circuit
	// before any HTTP call happens.

	mgr := &bootstrapRecordingMgr{}
	homeDir := t.TempDir()
	hook := func(ctx context.Context) {
		opts := bootstrap.Options{
			BinaryPath: "/test/bin/squad",
			Bind:       "127.0.0.1",
			Port:       7777,
			HomeDir:    homeDir,
			Manager:    mgr,
			Version:    "test-version",
		}
		_ = bootstrap.Ensure(ctx, opts)
		_ = bootstrap.Welcome(ctx, opts)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out, WithBootstrap(hook)); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	if got := mgr.installCalls.Load(); got != 0 {
		t.Errorf("Install called %d times under SQUAD_NO_AUTO_DAEMON=1, want 0", got)
	}
	if !strings.Contains(out.String(), `"id":1`) {
		t.Errorf("MCP did not answer initialize: out=%q", out.String())
	}
}

// TestMCP_RealBootstrap_NoBrowserEnv_WritesSentinelNoOpener asserts
// SQUAD_NO_BROWSER=1 still writes the welcome sentinel (so the
// once-per-machine invariant holds even without a UI) but does not
// invoke the opener.
func TestMCP_RealBootstrap_NoBrowserEnv_WritesSentinelNoOpener(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { _ = bootstrap.ConsumeBanner() })
	_ = bootstrap.ConsumeBanner()

	t.Setenv("SQUAD_NO_BROWSER", "1")

	bin := "/test/bin/squad"
	const ver = "test-version"
	srv := httptest.NewServer(versionResponder(ver, bin))
	t.Cleanup(srv.Close)
	t.Cleanup(bootstrap.SetProbeBaseForTest(srv.URL))

	homeDir := t.TempDir()
	mgr := &bootstrapRecordingMgr{}
	var openerCalls atomic.Int32
	hook := func(ctx context.Context) {
		opts := bootstrap.Options{
			BinaryPath: bin,
			Bind:       "127.0.0.1",
			Port:       7777,
			HomeDir:    homeDir,
			Manager:    mgr,
			Version:    ver,
			Opener:     func(string) error { openerCalls.Add(1); return nil },
		}
		_ = bootstrap.Ensure(ctx, opts)
		_ = bootstrap.Welcome(ctx, opts)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out, WithBootstrap(hook)); err != nil {
		t.Fatalf("runMCP: %v", err)
	}
	if got := openerCalls.Load(); got != 0 {
		t.Errorf("opener called %d times under SQUAD_NO_BROWSER=1, want 0", got)
	}
	if _, err := os.Stat(homeDir + "/.squad/.welcomed"); err != nil {
		t.Errorf("sentinel not written under SQUAD_NO_BROWSER=1: %v", err)
	}
}

// TestMCP_RealBootstrap_InstallFailure_MCPStillServes covers the
// production invariant from the spec: bootstrap failures never strand
// the MCP server. Manager.Install errors out → bootstrap.Ensure returns
// a wrapped error → realBootstrap-style hook logs and continues → MCP
// still answers initialize and tools/call.
func TestMCP_RealBootstrap_InstallFailure_MCPStillServes(t *testing.T) {
	env := newTestEnv(t)
	t.Cleanup(func() { _ = bootstrap.ConsumeBanner() })
	_ = bootstrap.ConsumeBanner()

	// Probe target returns 404 forever so Ensure goes into the install
	// branch and Manager.Install fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(bootstrap.SetProbeBaseForTest(srv.URL))

	t.Setenv("SQUAD_NO_BROWSER", "1")
	mgr := &failingMgr{err: errors.New("plist permission denied")}
	homeDir := t.TempDir()

	var ensureErr error
	hook := func(ctx context.Context) {
		opts := bootstrap.Options{
			BinaryPath: "/test/bin/squad",
			Bind:       "127.0.0.1",
			Port:       7777,
			HomeDir:    homeDir,
			Manager:    mgr,
			Version:    "test-version",
		}
		ensureErr = bootstrap.Ensure(ctx, opts)
		// realBootstrap returns early on Ensure error — mirror that
		// here so the test exercises the production shape.
	}

	in := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"squad_whoami","arguments":{}}}` + "\n")
	var out bytes.Buffer
	if err := runMCP(context.Background(), env.DB, env.RepoID, env.Root, in, &out, WithBootstrap(hook)); err != nil {
		t.Fatalf("runMCP must keep serving on bootstrap failure; got %v", err)
	}
	if ensureErr == nil {
		t.Fatal("bootstrap.Ensure must surface a wrapped error on Install failure")
	}
	if !strings.Contains(ensureErr.Error(), "plist permission denied") {
		t.Errorf("error %q does not wrap install cause", ensureErr)
	}
	if got := mgr.installCalls.Load(); got != 1 {
		t.Errorf("Install calls=%d, want 1", got)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d:\n%s", len(lines), out.String())
	}
	if !strings.Contains(lines[0], `"id":1`) {
		t.Errorf("initialize response missing or malformed: %q", lines[0])
	}
	if !strings.Contains(lines[1], `"id":2`) {
		t.Errorf("tools/call response missing or malformed: %q", lines[1])
	}
}
