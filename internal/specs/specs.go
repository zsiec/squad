// Package specs parses .squad/specs/<name>.md frontmatter. The filename slug
// is the reference key; there is no spec ID.
package specs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Name        string   `yaml:"-"`
	Title       string   `yaml:"title"`
	Motivation  string   `yaml:"motivation"`
	Acceptance  []string `yaml:"acceptance"`
	NonGoals    []string `yaml:"non_goals"`
	Integration []string `yaml:"integration"`

	Body string `yaml:"-"`
	Path string `yaml:"-"`
}

var (
	frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)
	utf8BOM       = []byte{0xEF, 0xBB, 0xBF}
)

// Parse reads a spec file. Rejects "prd:" / "PRD:" keys so users migrating
// from ccpm get a fast error instead of silent degradation.
func Parse(path string) (Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	if bytes.HasPrefix(data, utf8BOM) {
		data = data[len(utf8BOM):]
	}
	m := frontmatterRe.FindSubmatch(data)
	if m == nil {
		return Spec{}, fmt.Errorf("no frontmatter in %s", path)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(m[1], &raw); err != nil {
		return Spec{}, fmt.Errorf("parse spec %s: %w", path, err)
	}
	for _, banned := range []string{"prd", "PRD"} {
		if _, found := raw[banned]; found {
			return Spec{}, fmt.Errorf("spec %s uses %q key — squad uses 'spec'", path, banned)
		}
	}
	var s Spec
	if err := yaml.Unmarshal(m[1], &s); err != nil {
		return Spec{}, fmt.Errorf("decode spec %s: %w", path, err)
	}
	if strings.TrimSpace(s.Title) == "" {
		return Spec{}, fmt.Errorf("spec %s missing title", path)
	}
	s.Path = path
	s.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	s.Body = string(data[len(m[0]):])
	return s, nil
}

// Walk reads every .md file in <squadDir>/specs/ and returns successfully
// parsed specs. Malformed files are skipped silently; doctor surfaces them.
func Walk(squadDir string) ([]Spec, error) {
	entries, err := os.ReadDir(filepath.Join(squadDir, "specs"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Spec
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if s, err := Parse(filepath.Join(squadDir, "specs", e.Name())); err == nil {
			out = append(out, s)
		}
	}
	return out, nil
}
