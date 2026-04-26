package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// installServiceFlow generates a fresh bearer token, writes it to
// ~/.squad/token (0600), and asks the platform Manager to install the
// service. The TUI's first-launch flow and `squad serve --install-service`
// both go through here so the on-disk token always matches what the
// service actually advertises.
func installServiceFlow(homeDir, binary string, mgr daemon.Manager) error {
	token := generateToken()
	tokenPath := filepath.Join(homeDir, ".squad", "token")
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
		LogDir:     filepath.Join(homeDir, ".squad", "logs"),
		HomeDir:    homeDir,
	})
}

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the host has no entropy source; the
		// caller can't paper over that, so panic rather than ship a
		// zero-bytes "token" that would still pass empty-string checks.
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return hex.EncodeToString(b)
}
