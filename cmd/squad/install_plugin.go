package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	squad "github.com/zsiec/squad"
	"github.com/zsiec/squad/internal/installer"
)

func newInstallPluginCmd() *cobra.Command {
	var (
		uninstall      bool
		registerMCP    bool
		printMCPConfig bool
		skipHooks      bool
	)
	cmd := &cobra.Command{
		Use:   "install-plugin",
		Short: "Install the squad Claude Code plugin to ~/.claude/plugins/squad/",
		RunE: func(cmd *cobra.Command, args []string) error {
			if printMCPConfig {
				return printMCPSnippet(cmd.OutOrStdout())
			}

			dst, err := pluginDestDir()
			if err != nil {
				return err
			}
			pluginDir := filepath.Join(dst, "squad")

			if uninstall {
				if err := installer.Uninstall(pluginDir); err != nil {
					return fmt.Errorf("uninstall plugin at %s: %w", pluginDir, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "uninstalled squad plugin from %s\n", pluginDir)
				if registerMCP {
					settingsPath, err := defaultSettingsPath()
					if err != nil {
						return err
					}
					if err := installer.UnmergeMCPServers(settingsPath, []string{"squad"}); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "squad: warning: could not unmerge mcpServers from %s: %v\n", settingsPath, err)
					}
				}
				return nil
			}

			assets, err := squad.PluginFS()
			if err != nil {
				return fmt.Errorf("load embedded plugin assets: %w", err)
			}
			if err := installer.Install(assets, pluginDir); err != nil {
				return fmt.Errorf("install plugin to %s: %w", pluginDir, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed squad plugin to %s\n", pluginDir)

			if registerMCP {
				settingsPath, err := defaultSettingsPath()
				if err != nil {
					return err
				}
				if err := installer.MergeMCPServers(settingsPath, squadMCPSpec()); err != nil {
					return fmt.Errorf("register MCP server in %s: %w", settingsPath, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "registered squad MCP server in %s\n", settingsPath)
			}

			if !skipHooks {
				settingsPath, err := defaultSettingsPath()
				if err != nil {
					return err
				}
				enabled := map[string]bool{
					"session-start":    true,
					"user-prompt-tick": true,
					"pre-compact":      true,
				}
				if err := mergeSquadHooks(settingsPath, enabled); err != nil {
					return fmt.Errorf("register hooks in %s: %w", settingsPath, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "registered always-on hooks in %s\n", settingsPath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the squad plugin instead of installing")
	cmd.Flags().BoolVar(&registerMCP, "register-mcp", true, "merge mcpServers.squad into ~/.claude/settings.json")
	cmd.Flags().BoolVar(&printMCPConfig, "print-mcp-config", false, "print the JSON snippet that would be merged and exit")
	cmd.Flags().BoolVar(&skipHooks, "skip-hooks", false, "do not register the always-on session/prompt/pre-compact hooks")
	return cmd
}

func squadMCPSpec() map[string]installer.MCPServerSpec {
	return map[string]installer.MCPServerSpec{
		"squad": {
			Command:       "squad",
			Args:          []string{"mcp"},
			Env:           map[string]string{"SQUAD_MCP_SOURCE": "install-plugin"},
			MarkerVersion: versionString,
		},
	}
}

func printMCPSnippet(out io.Writer) error {
	snippet := map[string]any{"mcpServers": map[string]any{}}
	for name, spec := range squadMCPSpec() {
		entry := map[string]any{
			"command": spec.Command,
			"args":    spec.Args,
			"squad":   spec.MarkerVersion,
		}
		if len(spec.Env) > 0 {
			env := map[string]any{}
			for k, v := range spec.Env {
				env[k] = v
			}
			entry["env"] = env
		}
		snippet["mcpServers"].(map[string]any)[name] = entry
	}
	raw, err := json.MarshalIndent(snippet, "", "  ")
	if err != nil {
		return err
	}
	_, err = out.Write(append(raw, '\n'))
	return err
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
