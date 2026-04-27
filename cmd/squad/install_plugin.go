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
	"github.com/zsiec/squad/internal/tui/daemon"
	"github.com/zsiec/squad/plugin/hooks"
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
				if registerMCP || !skipHooks {
					settingsPath, err := defaultSettingsPath()
					if err != nil {
						return err
					}
					if registerMCP {
						if err := installer.UnmergeMCPServers(settingsPath, []string{"squad"}); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "squad: warning: could not unmerge mcpServers from %s: %v\n", settingsPath, err)
						}
					}
					if !skipHooks {
						if err := uninstallSquadHooks(settingsPath); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "squad: warning: could not unregister hooks from %s: %v\n", settingsPath, err)
						} else if dir, err := materializedHooksDir(); err == nil {
							if err := os.RemoveAll(dir); err != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "squad: warning: could not remove materialized hooks dir %s: %v\n", dir, err)
							}
						}
					}
				}
				if err := daemon.New().Uninstall(); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "squad: warning: could not uninstall background daemon: %v\n", err)
				}
				return nil
			}

			legacy := filepath.Join(pluginDir, "plugin.json")
			if _, err := os.Stat(legacy); err == nil {
				fmt.Fprintf(cmd.OutOrStdout(),
					"legacy plugin.json detected at %s; the new layout will replace it.\n"+
						"  if any external configs reference it, update to %s.\n",
					legacy, filepath.Join(pluginDir, ".claude-plugin", "plugin.json"))
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
				enabled := defaultOnHooks()
				if err := mergeSquadHooks(settingsPath, enabled); err != nil {
					return fmt.Errorf("register hooks in %s: %w", settingsPath, err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "registered %d default hooks in %s\n", len(enabled), settingsPath)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the squad plugin instead of installing")
	cmd.Flags().BoolVar(&registerMCP, "register-mcp", true, "merge mcpServers.squad into ~/.claude/settings.json")
	cmd.Flags().BoolVar(&printMCPConfig, "print-mcp-config", false, "print the JSON snippet that would be merged and exit")
	cmd.Flags().BoolVar(&skipHooks, "skip-hooks", false, "do not register the default-on hooks (use squad install-hooks for fine-grained control)")
	return cmd
}

// defaultOnHooks returns the set of squad hook names that should be installed
// by default. Single source of truth: plugin/hooks.embed.go's All array
// filtered to DefaultOn==true. install-hooks consumes the same array, so
// install-plugin and install-hooks --yes always agree.
func defaultOnHooks() map[string]bool {
	out := make(map[string]bool, len(hooks.All))
	for _, h := range hooks.All {
		if h.DefaultOn {
			out[h.Name] = true
		}
	}
	return out
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
