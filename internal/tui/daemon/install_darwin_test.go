//go:build darwin

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
}

func (f *fakeExec) Run(name string, args ...string) error {
	f.runCalls = append(f.runCalls, name+" "+strings.Join(args, " "))
	return f.runErr
}
func (f *fakeExec) Output(name string, args ...string) ([]byte, error) {
	cmd := name + " " + strings.Join(args, " ")
	f.outputCalls = append(f.outputCalls, cmd)
	if f.outputData != nil {
		if data, ok := f.outputData[cmd]; ok {
			return []byte(data), nil
		}
	}
	return nil, fmt.Errorf("no fake output for %q", cmd)
}

func TestDarwin_InstallWritesPlistAndCallsLaunchctl(t *testing.T) {
	tmp := t.TempDir()
	fe := &fakeExec{}
	m := newWithExec(tmp, fe)
	err := m.Install(InstallOpts{
		BinaryPath: "/usr/local/bin/squad",
		Bind:       "127.0.0.1",
		Port:       7777,
		LogDir:     filepath.Join(tmp, ".squad/logs"),
		HomeDir:    tmp,
	})
	if err != nil {
		t.Fatal(err)
	}

	p := filepath.Join(tmp, "Library", "LaunchAgents", "sh.squad.serve.plist")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"sh.squad.serve",
		"/usr/local/bin/squad",
		"127.0.0.1",
		"7777",
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("plist missing %q: %s", want, data)
		}
	}
	if len(fe.runCalls) == 0 || !strings.Contains(fe.runCalls[0], "launchctl bootstrap") {
		t.Errorf("expected launchctl bootstrap call, got %v", fe.runCalls)
	}
}

func TestDarwin_UninstallRemovesPlistAndCallsBootout(t *testing.T) {
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
	p := filepath.Join(tmp, "Library", "LaunchAgents", "sh.squad.serve.plist")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("plist still exists after uninstall")
	}
	if len(fe.runCalls) == 0 || !strings.Contains(fe.runCalls[0], "launchctl bootout") {
		t.Errorf("expected launchctl bootout, got %v", fe.runCalls)
	}
}

func TestDarwin_StatusReportsNotInstalledWhenNoPlist(t *testing.T) {
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

func TestDarwin_StatusReportsInstalledAndRunning(t *testing.T) {
	tmp := t.TempDir()
	fe := &fakeExec{
		outputData: map[string]string{
			"launchctl list sh.squad.serve": `{
    "PID" = 12345;
    "Label" = "sh.squad.serve";
    "Program" = "/usr/local/bin/squad";
};
`,
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
	if s.PID != 12345 {
		t.Errorf("PID=%d want 12345", s.PID)
	}
}

func TestDarwin_ReinstallUninstallsThenInstalls(t *testing.T) {
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
	p := filepath.Join(tmp, "Library", "LaunchAgents", "sh.squad.serve.plist")
	data, _ := os.ReadFile(p)
	if !bytes.Contains(data, []byte("/new/squad")) {
		t.Errorf("plist not updated to new binary path: %s", data)
	}
}
