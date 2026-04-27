package main

import (
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

func TestInstallServiceFlow_CallsManagerWithExpectedOpts(t *testing.T) {
	tmp := t.TempDir()
	mgr := &recordingMgr{}
	if err := installServiceFlow(tmp, "/p/squad", mgr); err != nil {
		t.Fatal(err)
	}
	if mgr.installCalls != 1 {
		t.Errorf("install called %d times", mgr.installCalls)
	}
	if mgr.lastOpts.BinaryPath != "/p/squad" {
		t.Errorf("opts.BinaryPath=%q", mgr.lastOpts.BinaryPath)
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
