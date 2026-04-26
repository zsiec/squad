package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeMCPServers_FreshSettings(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")
	if err := MergeMCPServers(settingsPath, map[string]MCPServerSpec{
		"squad": {Command: "squad", Args: []string{"mcp"}, MarkerVersion: "0.2.0"},
	}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(settingsPath)
	var got struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
			Squad   string   `json:"squad"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	sq := got.MCPServers["squad"]
	if sq.Command != "squad" {
		t.Errorf("command=%q", sq.Command)
	}
	if sq.Squad != "0.2.0" {
		t.Errorf("marker=%q", sq.Squad)
	}
}

func TestMergeMCPServers_PreservesOtherServers(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")
	seed := `{"mcpServers":{"otherTool":{"command":"foo","args":["bar"]}},"theme":"dark"}`
	if err := os.WriteFile(settingsPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := MergeMCPServers(settingsPath, map[string]MCPServerSpec{
		"squad": {Command: "squad", Args: []string{"mcp"}, MarkerVersion: "0.2.0"},
	}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(settingsPath)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["theme"] != "dark" {
		t.Errorf("lost top-level field; got %v", got["theme"])
	}
	servers := got["mcpServers"].(map[string]any)
	if _, ok := servers["otherTool"]; !ok {
		t.Error("clobbered otherTool")
	}
	if _, ok := servers["squad"]; !ok {
		t.Error("squad not added")
	}
}

func TestMergeMCPServers_IdempotentReinstall(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")
	spec := map[string]MCPServerSpec{
		"squad": {Command: "squad", Args: []string{"mcp"}, MarkerVersion: "0.2.0"},
	}
	for i := 0; i < 3; i++ {
		if err := MergeMCPServers(settingsPath, spec); err != nil {
			t.Fatalf("merge %d: %v", i, err)
		}
	}
	raw, _ := os.ReadFile(settingsPath)
	if !json.Valid(raw) {
		t.Fatal("settings.json corrupted after repeat merges")
	}
}

func TestUnmergeMCPServers_RemovesMarkedEntry(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")
	if err := MergeMCPServers(settingsPath, map[string]MCPServerSpec{
		"squad": {Command: "squad", Args: []string{"mcp"}, MarkerVersion: "0.2.0"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := UnmergeMCPServers(settingsPath, []string{"squad"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(settingsPath)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	servers, _ := got["mcpServers"].(map[string]any)
	if _, ok := servers["squad"]; ok {
		t.Error("expected squad entry removed")
	}
}

func TestUnmergeMCPServers_LeavesHandEdited(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")
	seed := `{"mcpServers":{"squad":{"command":"foo","args":["bar"]}}}`
	if err := os.WriteFile(settingsPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := UnmergeMCPServers(settingsPath, []string{"squad"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(settingsPath)
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	servers, _ := got["mcpServers"].(map[string]any)
	if _, ok := servers["squad"]; !ok {
		t.Error("hand-edited entry without marker should be preserved")
	}
}

func TestUnmergeMCPServers_MissingFileNoError(t *testing.T) {
	tmp := t.TempDir()
	settingsPath := filepath.Join(tmp, "settings.json")
	if err := UnmergeMCPServers(settingsPath, []string{"squad"}); err != nil {
		t.Fatalf("unmerge on missing file should be a no-op: %v", err)
	}
}
