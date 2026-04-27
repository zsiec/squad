package main

import (
	"path/filepath"

	"github.com/zsiec/squad/internal/tui/daemon"
)

// installServiceFlow asks the platform Manager to install the squad serve
// service. The TUI's first-launch flow and `squad serve --install-service`
// both go through here.
func installServiceFlow(homeDir, binary string, mgr daemon.Manager) error {
	return mgr.Install(daemon.InstallOpts{
		BinaryPath: binary,
		Bind:       "127.0.0.1",
		Port:       7777,
		LogDir:     filepath.Join(homeDir, ".squad", "logs"),
		HomeDir:    homeDir,
	})
}
