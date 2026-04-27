package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// daemonArtifactPath returns the on-disk path the daemon Manager would write
// for the current OS. Mirrors install_darwin.go / install_linux.go in the
// daemon package — they're not exported so the test re-derives them.
func daemonArtifactPath(home string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "LaunchAgents", "sh.squad.serve.plist")
	case "linux":
		return filepath.Join(home, ".config", "systemd", "user", "squad-serve.service")
	default:
		return ""
	}
}

func TestInstallPlugin_UninstallRemovesDaemonPreservesWelcomedAndDB(t *testing.T) {
	artifact := daemonArtifactPath("")
	if artifact == "" {
		t.Skipf("daemon Manager not implemented on %s", runtime.GOOS)
	}

	tmp := t.TempDir()
	t.Setenv("SQUAD_PLUGIN_DEST", filepath.Join(tmp, "plugins"))
	t.Setenv("HOME", tmp)
	t.Setenv("SQUAD_HOME", filepath.Join(tmp, ".squad"))

	cmd := newInstallPluginCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}

	artifactPath := daemonArtifactPath(tmp)
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte("simulated daemon artifact"), 0o644); err != nil {
		t.Fatal(err)
	}

	squadHome := filepath.Join(tmp, ".squad")
	if err := os.MkdirAll(squadHome, 0o755); err != nil {
		t.Fatal(err)
	}
	welcomed := filepath.Join(squadHome, ".welcomed")
	globalDB := filepath.Join(squadHome, "global.db")
	if err := os.WriteFile(welcomed, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	dbBytes := []byte("SIMULATED-DB-CONTENT-DO-NOT-DELETE")
	if err := os.WriteFile(globalDB, dbBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd2 := newInstallPluginCmd()
	cmd2.SetOut(&bytes.Buffer{})
	cmd2.SetErr(&bytes.Buffer{})
	cmd2.SetArgs([]string{"--uninstall"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("uninstall: %v", err)
	}

	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Errorf("daemon artifact at %s should be gone, err=%v", artifactPath, err)
	}
	if _, err := os.Stat(welcomed); err != nil {
		t.Errorf(".welcomed must be preserved across uninstall (welcome state outlives plugin install): %v", err)
	}
	got, err := os.ReadFile(globalDB)
	if err != nil {
		t.Fatalf("global.db should be preserved: %v", err)
	}
	if !bytes.Equal(got, dbBytes) {
		t.Errorf("global.db content corrupted: got %q want %q", got, dbBytes)
	}
	if _, err := os.Stat(squadHome); err != nil {
		t.Errorf("$SQUAD_HOME dir should be preserved: %v", err)
	}
}

func TestInstallPlugin_UninstallOnBareHomeIsNotAnError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("SQUAD_PLUGIN_DEST", filepath.Join(tmp, "plugins"))
	t.Setenv("HOME", tmp)
	t.Setenv("SQUAD_HOME", filepath.Join(tmp, ".squad"))

	cmd := newInstallPluginCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--uninstall"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("uninstall on bare HOME should not error: %v", err)
	}
}
