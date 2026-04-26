package skills

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSquadLoopSkill_RegistersScopedHook(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "plugin", "skills", "squad-loop", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	fm, _, err := splitFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Hooks []struct {
			Event   string `yaml:"event"`
			Matcher string `yaml:"matcher"`
			Run     string `yaml:"run"`
		} `yaml:"hooks"`
	}
	if err := yaml.Unmarshal(fm, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Hooks) == 0 {
		t.Fatal("squad-loop should declare at least one scoped hook")
	}
	var hasPreToolBash bool
	for _, h := range f.Hooks {
		if h.Event == "PreToolUse" && h.Matcher == "Bash" {
			hasPreToolBash = true
		}
	}
	if !hasPreToolBash {
		t.Error("expected PreToolUse:Bash scoped hook")
	}
}
