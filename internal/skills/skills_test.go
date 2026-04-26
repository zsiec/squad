package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type Frontmatter struct {
	Name                   string   `yaml:"name"`
	Description            string   `yaml:"description"`
	AllowedTools           []string `yaml:"allowed-tools"`
	Paths                  []string `yaml:"paths"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation"`
	Hooks                  []string `yaml:"hooks"`
}

func TestEverySkillHasValidFrontmatter(t *testing.T) {
	root := filepath.Join("..", "..", "plugin", "skills")
	var dirs []string
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) < 9 {
		t.Fatalf("expected at least 9 skill dirs, got %d (%v)", len(dirs), dirs)
	}
	for _, d := range dirs {
		path := filepath.Join(root, d, "SKILL.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing SKILL.md for %s: %v", d, err)
			continue
		}
		fm, body, err := splitFrontmatter(raw)
		if err != nil {
			t.Errorf("%s: split frontmatter: %v", d, err)
			continue
		}
		if len(strings.TrimSpace(string(body))) == 0 {
			t.Errorf("%s: SKILL.md body is empty", d)
		}
		var f Frontmatter
		if err := yaml.Unmarshal(fm, &f); err != nil {
			t.Errorf("%s: invalid yaml: %v", d, err)
			continue
		}
		if f.Name == "" {
			t.Errorf("%s: name missing", d)
		}
		if f.Description == "" {
			t.Errorf("%s: description missing", d)
		}
		if f.Name != d {
			t.Errorf("%s: name %q does not match dir name", d, f.Name)
		}
	}
}
