package main

import (
	"bytes"
	"context"
	"encoding/json"
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

func (m *bootstrapRecordingMgr) Install(daemon.InstallOpts) error   { m.installCalls.Add(1); return nil }
func (m *bootstrapRecordingMgr) Uninstall() error                   { return nil }
func (m *bootstrapRecordingMgr) Status() (daemon.Status, error)     { return daemon.Status{}, nil }
func (m *bootstrapRecordingMgr) Reinstall(daemon.InstallOpts) error { m.reinstallCalls.Add(1); return nil }

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
