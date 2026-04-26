//go:build darwin

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

const launchdLabel = "sh.squad.serve"

const plistTpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key><string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
      <string>{{.BinaryPath}}</string>
      <string>serve</string>
      <string>--bind</string><string>{{.Bind}}</string>
      <string>--port</string><string>{{.Port}}</string>
      <string>--token</string><string>{{.Token}}</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key>
    <dict><key>SuccessfulExit</key><false/></dict>
    <key>StandardOutPath</key><string>{{.LogDir}}/serve.out.log</string>
    <key>StandardErrorPath</key><string>{{.LogDir}}/serve.err.log</string>
    <key>EnvironmentVariables</key>
    <dict><key>SQUAD_HOME</key><string>{{.HomeDir}}/.squad</string></dict>
    <key>ProcessType</key><string>Background</string>
  </dict>
</plist>
`

type darwinManager struct {
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

// New returns the darwin Manager rooted at the current user's home
// directory. Tests use newWithExec to swap out the home and exec sink.
func New() Manager {
	home, _ := os.UserHomeDir()
	return &darwinManager{home: home, exec: realExec{}}
}

func newWithExec(home string, e execer) *darwinManager {
	return &darwinManager{home: home, exec: e}
}

func (m *darwinManager) plistPath() string {
	return filepath.Join(m.home, "Library", "LaunchAgents", launchdLabel+".plist")
}

func (m *darwinManager) Install(opts InstallOpts) error {
	var buf bytes.Buffer
	tpl := template.Must(template.New("plist").Parse(plistTpl))
	err := tpl.Execute(&buf, struct {
		Label, BinaryPath, Bind, Port, Token, HomeDir, LogDir string
	}{
		Label: launchdLabel, BinaryPath: opts.BinaryPath,
		Bind: opts.Bind, Port: fmt.Sprintf("%d", opts.Port),
		Token: opts.Token, HomeDir: opts.HomeDir, LogDir: opts.LogDir,
	})
	if err != nil {
		return fmt.Errorf("render plist: %w", err)
	}
	p := m.plistPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir LaunchAgents: %w", err)
	}
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
		return fmt.Errorf("mkdir logs: %w", err)
	}
	uid := os.Getuid()
	if err := m.exec.Run("launchctl", "bootstrap", fmt.Sprintf("gui/%d", uid), p); err != nil {
		return fmt.Errorf("launchctl bootstrap: %w", err)
	}
	return nil
}

func (m *darwinManager) Uninstall() error {
	p := m.plistPath()
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil
	}
	uid := os.Getuid()
	_ = m.exec.Run("launchctl", "bootout", fmt.Sprintf("gui/%d", uid), p)
	return os.Remove(p)
}

func (m *darwinManager) Status() (Status, error) {
	p := m.plistPath()
	s := Status{}
	if _, err := os.Stat(p); err != nil {
		return s, nil
	}
	s.Installed = true
	out, err := m.exec.Output("launchctl", "list", launchdLabel)
	if err == nil && len(out) > 0 {
		s.Running = true
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "\"PID\"") {
				rest := strings.TrimPrefix(trimmed, "\"PID\" = ")
				rest = strings.TrimSuffix(rest, ";")
				_, _ = fmt.Sscanf(rest, "%d", &s.PID)
			}
		}
	}
	return s, nil
}

func (m *darwinManager) Reinstall(opts InstallOpts) error {
	if err := m.Uninstall(); err != nil {
		return err
	}
	return m.Install(opts)
}
