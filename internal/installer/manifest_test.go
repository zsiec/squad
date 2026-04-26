package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestManifest_HasRequiredFields(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "plugin", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	required := []string{
		"name", "version", "description", "author", "license",
		"homepage", "repository", "min_claude_code_version",
		"skills", "commands", "agents", "hooks", "mcp_servers",
		"keywords", "engines",
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			t.Errorf("manifest missing required field: %s", k)
		}
	}
	if m["name"] != "squad" {
		t.Errorf("name should be squad, got %v", m["name"])
	}
}
