package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallPlugin_CreatesDestDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_PLUGIN_DEST", filepath.Join(tmp, "plugins"))

	var stdout, stderr bytes.Buffer
	cmd := newInstallPluginCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v\nstderr: %s", err, stderr.String())
	}

	manifest := filepath.Join(tmp, "plugins", "squad", "plugin.json")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("manifest should exist at %s: %v", manifest, err)
	}
}

func TestInstallPlugin_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_PLUGIN_DEST", filepath.Join(tmp, "plugins"))

	for i := 0; i < 2; i++ {
		cmd := newInstallPluginCmd()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("install pass %d: %v", i, err)
		}
	}
}

func TestInstallPlugin_Uninstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_PLUGIN_DEST", filepath.Join(tmp, "plugins"))

	cmd := newInstallPluginCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}

	pluginDir := filepath.Join(tmp, "plugins", "squad")
	if _, err := os.Stat(pluginDir); err != nil {
		t.Fatalf("plugin dir should exist after install: %v", err)
	}

	cmd2 := newInstallPluginCmd()
	cmd2.SetOut(&bytes.Buffer{})
	cmd2.SetErr(&bytes.Buffer{})
	cmd2.SetArgs([]string{"--uninstall"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Fatalf("plugin dir should be gone after uninstall, err=%v", err)
	}
}

func TestInstallPlugin_UninstallIdempotent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_PLUGIN_DEST", filepath.Join(tmp, "plugins"))

	cmd := newInstallPluginCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--uninstall"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("uninstall on absent should not error: %v", err)
	}
}
