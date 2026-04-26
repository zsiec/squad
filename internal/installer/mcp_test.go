package installer

import (
	"encoding/json"
	"io/fs"
	"testing"

	squad "github.com/zsiec/squad"
)

func TestPluginShipsMCPRegistration(t *testing.T) {
	src, err := squad.PluginFS()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := fs.ReadFile(src, ".mcp.json")
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	var m struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	sq, ok := m.MCPServers["squad"]
	if !ok {
		t.Fatal("missing 'squad' server entry")
	}
	if sq.Command == "" {
		t.Error("command empty")
	}
	if len(sq.Args) == 0 || sq.Args[len(sq.Args)-1] != "mcp" {
		t.Errorf("expected last arg to be 'mcp', got %v", sq.Args)
	}
}
