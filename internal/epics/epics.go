// Package epics parses .squad/epics/<name>.md frontmatter. Each epic
// references a spec by slug; Walk reports epics whose spec doesn't
// resolve to an actual file on disk (broken back-reference).
package epics

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/zsiec/squad/internal/specs"
)

type Epic struct {
	Name        string `yaml:"-"`
	Spec        string `yaml:"spec"`
	Status      string `yaml:"status"`
	Parallelism string `yaml:"parallelism"`

	Body string `yaml:"-"`
	Path string `yaml:"-"`
}

type Broken struct {
	Path  string
	Error string
}

var (
	frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)
	utf8BOM       = []byte{0xEF, 0xBB, 0xBF}
)

func Parse(path string) (Epic, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Epic{}, err
	}
	if bytes.HasPrefix(data, utf8BOM) {
		data = data[len(utf8BOM):]
	}
	m := frontmatterRe.FindSubmatch(data)
	if m == nil {
		return Epic{}, fmt.Errorf("no frontmatter in %s", path)
	}
	var e Epic
	if err := yaml.Unmarshal(m[1], &e); err != nil {
		return Epic{}, fmt.Errorf("parse epic %s: %w", path, err)
	}
	if strings.TrimSpace(e.Spec) == "" {
		return Epic{}, fmt.Errorf("epic %s has empty 'spec' field", path)
	}
	e.Path = path
	e.Name = strings.TrimSuffix(filepath.Base(path), ".md")
	if e.Status == "" {
		e.Status = "open"
	}
	e.Body = string(data[len(m[0]):])
	return e, nil
}

// Walk validates each epic's `spec:` against known specs. Broken
// back-references are surfaced separately so doctor/analyze can decide.
func Walk(squadDir string) ([]Epic, []Broken, error) {
	specList, err := specs.Walk(squadDir)
	if err != nil {
		return nil, nil, err
	}
	known := map[string]bool{}
	for _, s := range specList {
		known[s.Name] = true
	}

	dir := filepath.Join(squadDir, "epics")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var out []Epic
	var broken []Broken
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".md") {
			continue
		}
		if strings.HasSuffix(ent.Name(), "-analysis.md") {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		e, err := Parse(path)
		if err != nil {
			broken = append(broken, Broken{Path: path, Error: err.Error()})
			continue
		}
		if !known[e.Spec] {
			broken = append(broken, Broken{
				Path:  path,
				Error: fmt.Sprintf("references unknown spec %q", e.Spec),
			})
			continue
		}
		out = append(out, e)
	}
	return out, broken, nil
}
