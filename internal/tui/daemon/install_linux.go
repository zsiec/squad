//go:build linux

package daemon

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const systemdUnitName = "squad-serve"

const unitTpl = `[Unit]
Description=squad serve daemon
After=default.target

[Service]
Type=simple
ExecStart={{.BinaryPath}} serve --bind {{.Bind}} --port {{.Port}}
Restart=on-failure
RestartSec=2s
StandardOutput=append:{{.LogDir}}/serve.out.log
StandardError=append:{{.LogDir}}/serve.err.log
Environment=SQUAD_HOME={{.HomeDir}}/.squad
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:{{.HomeDir}}/.local/bin:{{.HomeDir}}/.claude/local:{{.HomeDir}}/go/bin

[Install]
WantedBy=default.target
`

type linuxManager struct {
	home string
	exec execer
}

type execer interface {
	Run(name string, args ...string) error
	Output(name string, args ...string) ([]byte, error)
}

type realExec struct{}

func (realExec) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}
func (realExec) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// New returns the linux Manager rooted at the current user's home
// directory. Tests use newWithExec to swap out the home and exec sink.
func New() Manager {
	home, _ := os.UserHomeDir()
	return &linuxManager{home: home, exec: realExec{}}
}

func newWithExec(home string, e execer) *linuxManager {
	return &linuxManager{home: home, exec: e}
}

func (m *linuxManager) unitPath() string {
	return filepath.Join(m.home, ".config", "systemd", "user", systemdUnitName+".service")
}

// systemdUserAvailable reports whether the user-bus socket is reachable
// at $XDG_RUNTIME_DIR/bus. When false, systemctl --user invocations would
// fail with "Failed to connect to bus". The Install path skips them and
// emits a warning to stderr instead of returning an error — the unit
// file is on disk and the user can load it manually if they want.
func systemdUserAvailable() bool {
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(runtimeDir, "bus")); err != nil {
		return false
	}
	return true
}

func (m *linuxManager) Install(opts InstallOpts) error {
	var buf bytes.Buffer
	tpl := template.Must(template.New("unit").Parse(unitTpl))
	err := tpl.Execute(&buf, struct {
		BinaryPath, Bind, Port, HomeDir, LogDir string
	}{
		BinaryPath: opts.BinaryPath,
		Bind:       opts.Bind, Port: fmt.Sprintf("%d", opts.Port),
		HomeDir: opts.HomeDir, LogDir: opts.LogDir,
	})
	if err != nil {
		return fmt.Errorf("render unit: %w", err)
	}
	p := m.unitPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir systemd user dir: %w", err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
		return fmt.Errorf("mkdir logs: %w", err)
	}
	if !systemdUserAvailable() {
		fmt.Fprintln(os.Stderr,
			"warning: systemd user bus unavailable (no $XDG_RUNTIME_DIR/bus); "+
				"unit file written to "+p+" but systemctl --user not invoked. "+
				"Run `systemctl --user daemon-reload && systemctl --user enable --now "+systemdUnitName+"` "+
				"manually if you want it active in this environment.")
		return nil
	}
	if err := m.exec.Run("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if err := m.exec.Run("systemctl", "--user", "enable", "--now", systemdUnitName); err != nil {
		return fmt.Errorf("systemctl enable: %w", err)
	}
	return nil
}

func (m *linuxManager) Uninstall() error {
	p := m.unitPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil
	}
	if systemdUserAvailable() {
		_ = m.exec.Run("systemctl", "--user", "disable", "--now", systemdUnitName)
	}
	if err := os.Remove(p); err != nil {
		return err
	}
	if systemdUserAvailable() {
		_ = m.exec.Run("systemctl", "--user", "daemon-reload")
	}
	return nil
}

func (m *linuxManager) Status() (Status, error) {
	p := m.unitPath()
	s := Status{}
	if _, err := os.Stat(p); err != nil {
		return s, nil
	}
	s.Installed = true
	out, err := m.exec.Output("systemctl", "--user", "is-active", systemdUnitName)
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		s.Running = true
	}
	return s, nil
}

func (m *linuxManager) Reinstall(opts InstallOpts) error {
	if err := m.Uninstall(); err != nil {
		return err
	}
	return m.Install(opts)
}
