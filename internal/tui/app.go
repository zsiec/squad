// Package tui hosts the bubbletea TUI for squad.
package tui

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
	"github.com/zsiec/squad/internal/tui/daemon"
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

// resolveServer returns the URL + token the TUI should use. If the
// token file is missing, prompt the user once to install squad serve
// as a per-user background service, then re-read the token. The prompt
// reads a single line from stdin; empty / Y / y are treated as yes.
func resolveServer(_ context.Context) (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("home dir: %w", err)
	}
	tokenPath := filepath.Join(home, ".squad", "token")
	if _, statErr := os.Stat(tokenPath); os.IsNotExist(statErr) {
		if err := promptAndInstall(home, tokenPath); err != nil {
			return "", "", err
		}
	}
	tokenBytes, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", tokenPath, err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	return "http://127.0.0.1:7777", token, nil
}

func promptAndInstall(home, tokenPath string) error {
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
	mgr := daemon.New()
	token := newToken()
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o755); err != nil {
		return fmt.Errorf("mkdir squad home: %w", err)
	}
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		return fmt.Errorf("write token: %w", err)
	}
	return mgr.Install(daemon.InstallOpts{
		BinaryPath: binary,
		Bind:       "127.0.0.1",
		Port:       7777,
		Token:      token,
		LogDir:     filepath.Join(home, ".squad", "logs"),
		HomeDir:    home,
	})
}

func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return hex.EncodeToString(b)
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
