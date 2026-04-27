package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestSquadHandoffSkill_EndsWithPerSurpriseMandate covers the handoff
// skill's per-surprise mandate: the file must end with the rule that
// each surprise bullet either gets filed via `squad learning quick`
// or has an explicit skip reason. The mandate is the load-bearing
// contract — its position at end-of-file is what makes it the
// authoritative rule over earlier soft guidance in the same skill.
func TestSquadHandoffSkill_EndsWithPerSurpriseMandate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "plugin", "skills", "squad-handoff", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	const mandate = "for each surprise bullet, file via `squad learning quick` or explain why not."
	if !strings.Contains(body, mandate) {
		t.Fatalf("SKILL.md missing mandate sentence %q", mandate)
	}
	tail := strings.TrimSpace(body)
	if !strings.HasSuffix(tail, mandate) {
		const tailWindow = 200
		shown := tail
		if len(shown) > tailWindow {
			shown = shown[len(shown)-tailWindow:]
		}
		t.Errorf("mandate must be the last sentence of the skill; tail was:\n...%s", shown)
	}
}

func TestSquadLoopSkill_RegistersScopedHook(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "plugin", "skills", "squad-loop", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	fm, _, err := splitFrontmatter(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Claude Code's plugin schema requires hooks to be a map keyed by event
	// name (PreToolUse, PostToolUse, etc.), each value an array of matcher
	// blocks containing nested hook commands. The earlier flat-array shape
	// (event/matcher/run sibling keys) was rejected by the plugin loader as
	// "expected record, received array".
	var f struct {
		Hooks map[string][]struct {
			Matcher string `yaml:"matcher"`
			Hooks   []struct {
				Type    string `yaml:"type"`
				Command string `yaml:"command"`
			} `yaml:"hooks"`
		} `yaml:"hooks"`
	}
	if err := yaml.Unmarshal(fm, &f); err != nil {
		t.Fatal(err)
	}
	preTool, ok := f.Hooks["PreToolUse"]
	if !ok || len(preTool) == 0 {
		t.Fatal("squad-loop should declare a PreToolUse scoped hook")
	}
	var hasBash bool
	for _, m := range preTool {
		if m.Matcher == "Bash" && len(m.Hooks) > 0 && m.Hooks[0].Command != "" {
			hasBash = true
		}
	}
	if !hasBash {
		t.Error("expected PreToolUse:Bash hook with a command")
	}
}
