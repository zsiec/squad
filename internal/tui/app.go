// Package tui hosts the bubbletea TUI for squad.
package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/daemon"
)

// Run is the entry point invoked by cmd/squad/tui.go.
func Run(ctx context.Context) error {
	url, err := resolveServer(ctx)
	if err != nil {
		return err
	}
	// Derive the actor's agent id locally — the same path `squad whoami`
	// takes. The earlier flow called /api/whoami over HTTP, which required
	// `squad serve` running AND the agent registered in the DB. CLI users
	// for whom `squad whoami` worked were getting "could not determine
	// agent identity" from the TUI.
	agentID, err := identity.AgentID()
	if err != nil || agentID == "" {
		return fmt.Errorf("could not determine agent identity: %w", err)
	}
	c := client.New(url).WithAgent(agentID)
	eventCh := c.SubscribeEvents(ctx)

	scope := detectScope()
	m := NewModel(c, eventCh, scope)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

// resolveServer returns the URL the TUI should use. If the daemon plist /
// unit file is missing, prompt the user once to install squad serve as a
// per-user background service. The prompt reads a single line from stdin;
// empty / Y / y are treated as yes.
func resolveServer(_ context.Context) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	mgr := daemon.New()
	st, err := mgr.Status()
	if errors.Is(err, daemon.ErrUnsupported) {
		return "", fmt.Errorf("squad TUI: dashboard daemon not supported on this platform; run `squad serve` manually if you want the UI")
	}
	if err == nil && !st.Installed {
		if err := promptAndInstall(home, mgr); err != nil {
			return "", err
		}
	}
	return "http://127.0.0.1:7777", nil
}

func promptAndInstall(home string, mgr daemon.Manager) error {
	fmt.Println("squad serve isn't running. Install it as a background service so the TUI can connect across reboots? [Y/n]")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "" && answer != "y" && answer != "yes" {
		return fmt.Errorf("squad serve not installed; run `squad serve --install-service` when ready")
	}
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve squad binary: %w", err)
	}
	return mgr.Install(daemon.InstallOpts{
		BinaryPath: binary,
		Bind:       "127.0.0.1",
		Port:       7777,
		LogDir:     filepath.Join(home, ".squad", "logs"),
		HomeDir:    home,
	})
}

func detectScope() string {
	wd, err := os.Getwd()
	if err != nil {
		return "all"
	}
	for dir := wd; dir != "/"; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".squad")); err == nil {
			return filepath.Base(dir)
		}
	}
	return "all"
}
