package installer

import (
	"encoding/json"
	"io/fs"
	"testing"

	squad "github.com/zsiec/squad"
)

func TestHooksJSONShape(t *testing.T) {
	src, err := squad.PluginFS()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := fs.ReadFile(src, "hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var h struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatal(err)
	}

	want := []string{"SessionStart", "UserPromptSubmit", "PreCompact", "Stop", "PostToolUse", "SessionEnd", "PreToolUse"}
	for _, ev := range want {
		if _, ok := h.Hooks[ev]; !ok {
			t.Errorf("missing event handler: %s", ev)
		}
	}
}
