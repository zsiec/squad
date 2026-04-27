package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zsiec/squad/internal/tui/daemon"
)

type recordingMgr struct {
	installCalls   int
	uninstallCalls int
	reinstallCalls int
	statusCalls    int
	lastOpts       daemon.InstallOpts
}

func (r *recordingMgr) Install(opts daemon.InstallOpts) error {
	r.installCalls++
	r.lastOpts = opts
	return nil
}
func (r *recordingMgr) Uninstall() error { r.uninstallCalls++; return nil }
func (r *recordingMgr) Status() (daemon.Status, error) {
	r.statusCalls++
	return daemon.Status{}, nil
}
func (r *recordingMgr) Reinstall(opts daemon.InstallOpts) error {
	r.reinstallCalls++
	r.lastOpts = opts
	return nil
}

func TestInstallServiceFlow_WritesTokenAndCallsManager(t *testing.T) {
	tmp := t.TempDir()
	mgr := &recordingMgr{}
	if err := installServiceFlow(tmp, "/p/squad", mgr); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(tmp, ".squad", "token")
	info, err := os.Stat(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("token mode=%o want 0o600", info.Mode().Perm())
	}
	if mgr.installCalls != 1 {
		t.Errorf("install called %d times", mgr.installCalls)
	}
	if mgr.lastOpts.BinaryPath != "/p/squad" {
		t.Errorf("opts.BinaryPath=%q", mgr.lastOpts.BinaryPath)
	}
	if mgr.lastOpts.Token == "" {
		t.Error("token not passed to manager")
	}
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(tokenBytes) != mgr.lastOpts.Token {
		t.Errorf("token-on-disk=%q does not match opts.Token=%q", tokenBytes, mgr.lastOpts.Token)
	}
	if mgr.lastOpts.Bind != "127.0.0.1" {
		t.Errorf("opts.Bind=%q want 127.0.0.1", mgr.lastOpts.Bind)
	}
	if mgr.lastOpts.Port != 7777 {
		t.Errorf("opts.Port=%d want 7777", mgr.lastOpts.Port)
	}
	wantLogDir := filepath.Join(tmp, ".squad", "logs")
	if mgr.lastOpts.LogDir != wantLogDir {
		t.Errorf("opts.LogDir=%q want %q", mgr.lastOpts.LogDir, wantLogDir)
	}
	if mgr.lastOpts.HomeDir != tmp {
		t.Errorf("opts.HomeDir=%q want %q", mgr.lastOpts.HomeDir, tmp)
	}
}

func TestInstallServiceFlow_WritesRestartToken(t *testing.T) {
	tmp := t.TempDir()
	mgr := &recordingMgr{}
	if err := installServiceFlow(tmp, "/p/squad", mgr); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(tmp, ".squad", "restart.token")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("restart token not written: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("restart token mode=%o want 0o600", info.Mode().Perm())
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// 32 bytes of crypto/rand encoded as hex => 64 ASCII chars.
	if len(b) != 64 {
		t.Errorf("restart token len=%d want 64 hex chars", len(b))
	}
}

func TestInstallServiceFlow_RestartTokenIsStableAcrossReinstall(t *testing.T) {
	tmp := t.TempDir()
	mgr := &recordingMgr{}
	if err := installServiceFlow(tmp, "/p/squad", mgr); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(tmp, ".squad", "restart.token")
	first, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := installServiceFlow(tmp, "/p/squad", mgr); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("restart token rotated on second install: %q -> %q", first, second)
	}
}

func TestInstallServiceFlow_GeneratesUniqueTokens(t *testing.T) {
	tmp1 := t.TempDir()
	tmp2 := t.TempDir()
	mgr1 := &recordingMgr{}
	mgr2 := &recordingMgr{}
	if err := installServiceFlow(tmp1, "/p/squad", mgr1); err != nil {
		t.Fatal(err)
	}
	if err := installServiceFlow(tmp2, "/p/squad", mgr2); err != nil {
		t.Fatal(err)
	}
	if mgr1.lastOpts.Token == mgr2.lastOpts.Token {
		t.Errorf("expected unique tokens, both got %q", mgr1.lastOpts.Token)
	}
	if len(mgr1.lastOpts.Token) < 32 {
		t.Errorf("token too short: len=%d", len(mgr1.lastOpts.Token))
	}
}
