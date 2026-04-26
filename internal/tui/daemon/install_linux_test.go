//go:build linux

package daemon

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeExec struct {
	runCalls    []string
	outputCalls []string
	outputData  map[string]string
	runErr      error
	outputErr   map[string]error
}

func (f *fakeExec) Run(name string, args ...string) error {
	f.runCalls = append(f.runCalls, name+" "+strings.Join(args, " "))
	return f.runErr
}
func (f *fakeExec) Output(name string, args ...string) ([]byte, error) {
	cmd := name + " " + strings.Join(args, " ")
	f.outputCalls = append(f.outputCalls, cmd)
	if f.outputErr != nil {
		if err, ok := f.outputErr[cmd]; ok {
			return nil, err
		}
	}
	if f.outputData != nil {
		if data, ok := f.outputData[cmd]; ok {
			return []byte(data), nil
		}
	}
	return nil, fmt.Errorf("no fake output for %q", cmd)
}

func TestLinux_InstallWritesUnitAndCallsSystemctl(t *testing.T) {
	tmp := t.TempDir()
	fe := &fakeExec{}
	m := newWithExec(tmp, fe)
	err := m.Install(InstallOpts{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		Token:      "secret",
		LogDir:     filepath.Join(tmp, ".squad/logs"),
		HomeDir:    tmp,
	})
	if err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(tmp, ".config", "systemd", "user", "squad-serve.service")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"/usr/local/bin/squad",
		"--bind 127.0.0.1",
		"--port 7777",
		"--token secret",
		"Restart=on-failure",
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("unit missing %q: %s", want, data)
		}
	}
	joined := strings.Join(fe.runCalls, "\n")
	for _, want := range []string{
		"systemctl --user daemon-reload",
		"systemctl --user enable --now squad-serve",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected systemctl call %q, got %v", want, fe.runCalls)
		}
	}
}

func TestLinux_UninstallRemovesUnitAndCallsDisable(t *testing.T) {
	tmp := t.TempDir()
	fe := &fakeExec{}
	m := newWithExec(tmp, fe)
	if err := m.Install(InstallOpts{HomeDir: tmp, LogDir: filepath.Join(tmp, ".squad/logs")}); err != nil {
		t.Fatal(err)
	}
	fe.runCalls = nil
	if err := m.Uninstall(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(tmp, ".config", "systemd", "user", "squad-serve.service")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("unit still exists after uninstall")
	}
	joined := strings.Join(fe.runCalls, "\n")
	if !strings.Contains(joined, "systemctl --user disable --now squad-serve") {
		t.Errorf("expected systemctl disable, got %v", fe.runCalls)
	}
}

func TestLinux_StatusReportsNotInstalledWhenNoUnit(t *testing.T) {
	tmp := t.TempDir()
	fe := &fakeExec{}
	m := newWithExec(tmp, fe)
	s, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if s.Installed {
		t.Errorf("expected installed=false, got %+v", s)
	}
}

func TestLinux_StatusReportsInstalledAndRunning(t *testing.T) {
	tmp := t.TempDir()
	fe := &fakeExec{
		outputData: map[string]string{
			"systemctl --user is-active squad-serve":  "active\n",
			"systemctl --user is-enabled squad-serve": "enabled\n",
		},
	}
	m := newWithExec(tmp, fe)
	if err := m.Install(InstallOpts{HomeDir: tmp, LogDir: filepath.Join(tmp, ".squad/logs")}); err != nil {
		t.Fatal(err)
	}
	s, err := m.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !s.Installed || !s.Running {
		t.Errorf("expected installed+running, got %+v", s)
	}
}

func TestLinux_Install_GracefulWhenNoUserBus(t *testing.T) {
	tmp := t.TempDir()
	// Force XDG_RUNTIME_DIR to a path with no `bus` socket — emulates a
	// container without systemd-user.
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tmp, "no-such-runtime"))

	fe := &fakeExec{}
	m := newWithExec(tmp, fe)
	err := m.Install(InstallOpts{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1", Port: 7777, Token: "tok",
		LogDir:  filepath.Join(tmp, ".squad/logs"),
		HomeDir: tmp,
	})
	if err != nil {
		t.Fatalf("expected nil error when no user bus, got %v", err)
	}

	// Unit file should be written
	unit := filepath.Join(tmp, ".config", "systemd", "user", "squad-serve.service")
	if _, err := os.Stat(unit); err != nil {
		t.Errorf("unit file should be written even without systemd: %v", err)
	}

	// systemctl should NOT have been called (graceful skip)
	for _, call := range fe.runCalls {
		if strings.Contains(call, "systemctl") {
			t.Errorf("systemctl should not be called without user bus, got %q", call)
		}
	}
}

func TestLinux_Install_CallsSystemctlWhenBusPresent(t *testing.T) {
	tmp := t.TempDir()
	// Set up a fake bus socket so the helper detects systemd-user.
	runtimeDir := filepath.Join(tmp, "runtime")
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	busPath := filepath.Join(runtimeDir, "bus")
	if f, err := os.Create(busPath); err != nil {
		t.Fatal(err)
	} else {
		f.Close()
	}
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)

	fe := &fakeExec{}
	m := newWithExec(tmp, fe)
	if err := m.Install(InstallOpts{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1", Port: 7777, Token: "tok",
		LogDir:  filepath.Join(tmp, ".squad/logs"),
		HomeDir: tmp,
	}); err != nil {
		t.Fatal(err)
	}
	// systemctl should have been called twice (daemon-reload + enable --now)
	systemctlCalls := 0
	for _, call := range fe.runCalls {
		if strings.Contains(call, "systemctl") {
			systemctlCalls++
		}
	}
	if systemctlCalls != 2 {
		t.Errorf("expected 2 systemctl calls when bus present, got %d (calls=%v)", systemctlCalls, fe.runCalls)
	}
}

func TestLinux_ReinstallUninstallsThenInstalls(t *testing.T) {
	tmp := t.TempDir()
	fe := &fakeExec{}
	m := newWithExec(tmp, fe)
	if err := m.Install(InstallOpts{HomeDir: tmp, LogDir: filepath.Join(tmp, ".squad/logs"), BinaryPath: "/old/squad"}); err != nil {
		t.Fatal(err)
	}
	fe.runCalls = nil
	if err := m.Reinstall(InstallOpts{HomeDir: tmp, LogDir: filepath.Join(tmp, ".squad/logs"), BinaryPath: "/new/squad"}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(tmp, ".config", "systemd", "user", "squad-serve.service")
	data, _ := os.ReadFile(p)
	if !bytes.Contains(data, []byte("/new/squad")) {
		t.Errorf("unit not updated to new binary path: %s", data)
	}
}
