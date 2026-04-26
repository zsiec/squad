// Package tui hosts the bubbletea TUI for squad.
package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
)

// Run is the entry point invoked by cmd/squad/tui.go.
func Run(ctx context.Context) error {
	url, token, err := resolveServer(ctx)
	if err != nil {
		return err
	}
	c := client.New(url, token)
	eventCh := c.SubscribeEvents(ctx)

	scope := detectScope()
	m := NewModel(c, eventCh, scope)

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

// resolveServer finds the squad serve URL and token. Phase B Task 26
// will replace this with the real daemon-probe + first-launch install
// flow. For now: read ~/.squad/token if present and assume localhost:7777.
func resolveServer(_ context.Context) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("home dir: %w", err)
	}
	tokenPath := filepath.Join(home, ".squad", "token")
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w (run squad serve --install-service first; this is a Task 26 placeholder)", tokenPath, err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	return "http://127.0.0.1:7777", token, nil
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
