package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	squad "github.com/zsiec/squad"
	"github.com/zsiec/squad/internal/installer"
)

func newInstallPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-plugin",
		Short: "Install the squad Claude Code plugin to ~/.claude/plugins/squad/",
		RunE: func(cmd *cobra.Command, args []string) error {
			dst, err := pluginDestDir()
			if err != nil {
				return err
			}
			pluginDir := filepath.Join(dst, "squad")
			assets, err := squad.PluginFS()
			if err != nil {
				return fmt.Errorf("load embedded plugin assets: %w", err)
			}
			if err := installer.Install(assets, pluginDir); err != nil {
				return fmt.Errorf("install plugin to %s: %w", pluginDir, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed squad plugin to %s\n", pluginDir)
			return nil
		},
	}
	return cmd
}

func pluginDestDir() (string, error) {
	if v := os.Getenv("SQUAD_PLUGIN_DEST"); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "plugins"), nil
}
