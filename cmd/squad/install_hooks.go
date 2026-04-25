package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/plugin/hooks"
)

func newInstallHooksCmd() *cobra.Command {
	var (
		yes       bool
		uninstall bool
		status    bool
	)
	flagsByHook := make(map[string]*string, len(hooks.All))

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install or update squad's Claude Code hooks in ~/.claude/settings.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			perHook := make(map[string]string)
			for name, ptr := range flagsByHook {
				if ptr != nil && *ptr != "" {
					perHook[name] = *ptr
				}
			}
			return runInstallHooksWithFlags(cmd.OutOrStdout(), cmd.ErrOrStderr(), yes, uninstall, status, perHook)
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "skip prompts; use defaults (only session-start ON)")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove all squad-managed hook entries")
	cmd.Flags().BoolVar(&status, "status", false, "print which squad hooks are currently installed")

	for _, h := range hooks.All {
		v := new(string)
		flagsByHook[h.Name] = v
		def := "off"
		if h.DefaultOn {
			def = "on"
		}
		cmd.Flags().StringVar(v, h.Name, "",
			fmt.Sprintf("%s (on|off; default %s)", h.Description, def))
	}
	return cmd
}

func runInstallHooks(args []string, stdout, stderr io.Writer) error {
	cmd := newInstallHooksCmd()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	return cmd.Execute()
}

func runInstallHooksWithFlags(stdout, stderr io.Writer, yes, uninstall, status bool, perHook map[string]string) error {
	settingsPath, err := defaultSettingsPath()
	if err != nil {
		return err
	}

	switch {
	case uninstall:
		if err := uninstallSquadHooks(settingsPath); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "squad hooks removed from", settingsPath)
		return nil
	case status:
		return printStatus(stdout, settingsPath)
	}

	enabled, err := resolveEnabled(stdout, stderr, yes, perHook)
	if err != nil {
		return err
	}
	if err := mergeSquadHooks(settingsPath, enabled); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "squad hooks installed in", settingsPath)
	for _, h := range hooks.All {
		state := "off"
		if enabled[h.Name] {
			state = "on"
		}
		fmt.Fprintf(stdout, "  %-22s %s\n", h.Name, state)
	}
	fmt.Fprintln(stdout, "\nEmergency disable: export SQUAD_NO_HOOKS=1")
	return nil
}

func promptHook(io.Writer, hooks.Hook) (bool, error) {
	return false, nil
}

func resolveEnabled(stdout, stderr io.Writer, yes bool, perHook map[string]string) (map[string]bool, error) {
	enabled := map[string]bool{}
	for _, h := range hooks.All {
		if v, ok := perHook[h.Name]; ok {
			switch strings.ToLower(v) {
			case "on", "true", "1", "yes":
				enabled[h.Name] = true
			case "off", "false", "0", "no":
				enabled[h.Name] = false
			default:
				return nil, fmt.Errorf("--%s: expected on|off, got %q", h.Name, v)
			}
			continue
		}
		if yes {
			enabled[h.Name] = h.DefaultOn
			continue
		}
		ans, err := promptHook(stdout, h)
		if err != nil {
			return nil, err
		}
		enabled[h.Name] = ans
	}
	return enabled, nil
}

func defaultSettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func printStatus(out io.Writer, path string) error {
	settings, err := loadSettings(path)
	if err != nil {
		return err
	}
	hooksRaw, _ := settings["hooks"].(map[string]any)

	installed := map[string]bool{}
	for _, raw := range hooksRaw {
		list, ok := raw.([]any)
		if !ok {
			continue
		}
		for _, e := range list {
			m, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if marker, ok := m["squad"].(string); ok {
				name, _, _ := strings.Cut(marker, "@")
				installed[name] = true
			}
		}
	}

	fmt.Fprintln(out, "squad hook status (settings.json:", path+")")
	for _, h := range hooks.All {
		state := "OFF"
		if installed[h.Name] {
			state = "ON"
		}
		fmt.Fprintf(out, "  %-22s %s   %s\n", h.Name, state, h.Description)
	}
	return nil
}
