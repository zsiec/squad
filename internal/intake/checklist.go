// Package intake holds the schema, checklist, session, and commit logic
// for the interview-driven intake flow. It is pure Go with no MCP or CLI
// dependencies — the surfaces in cmd/squad/ wrap it.
package intake

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed checklist.yaml
var defaultChecklistYAML []byte

const overrideFilename = "intake-checklist.yaml"

// Checklist is the per-shape required/optional field manifest used to drive
// the interview's still_required computation.
type Checklist struct {
	Shapes map[string]Shape `yaml:"shapes"`
}

type Shape struct {
	Description string   `yaml:"description"`
	Required    FieldSet `yaml:"required"`
	Optional    FieldSet `yaml:"optional"`
}

// FieldSet accepts two YAML shapes: a flat sequence (item_only) or a
// per-artifact map keyed by spec/epic/item (spec_epic_items). The walker
// methods flatten both into dotted field names like "spec.title".
type FieldSet struct {
	Flat        []string
	PerArtifact map[string][]string
}

func (fs *FieldSet) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.SequenceNode:
		var flat []string
		if err := node.Decode(&flat); err != nil {
			return err
		}
		fs.Flat = flat
		return nil
	case yaml.MappingNode:
		m := map[string][]string{}
		if err := node.Decode(&m); err != nil {
			return err
		}
		fs.PerArtifact = m
		return nil
	}
	return fmt.Errorf("intake checklist: required/optional must be a list or a map (got yaml kind %d)", node.Kind)
}

// LoadChecklist reads the checklist for a repo. It prefers
// <squadDir>/intake-checklist.yaml when present; otherwise it falls back
// to the embedded default. squadDir may be empty or non-existent — the
// embedded default is the safety net.
func LoadChecklist(squadDir string) (Checklist, error) {
	if squadDir != "" {
		path := filepath.Join(squadDir, overrideFilename)
		body, err := os.ReadFile(path)
		if err == nil {
			return parseChecklist(body)
		}
		if !os.IsNotExist(err) {
			return Checklist{}, fmt.Errorf("read %s: %w", path, err)
		}
	}
	return parseChecklist(defaultChecklistYAML)
}

func parseChecklist(body []byte) (Checklist, error) {
	var c Checklist
	if err := yaml.Unmarshal(body, &c); err != nil {
		return Checklist{}, fmt.Errorf("parse intake checklist: %w", err)
	}
	if len(c.Shapes) == 0 {
		return Checklist{}, fmt.Errorf("intake checklist: no shapes defined")
	}
	return c, nil
}

// Required returns the flat list of required field names for a shape.
// Per-artifact entries are dotted: "spec.title", "epic.parallelism", etc.
// Returns nil if the shape is unknown.
func (c Checklist) Required(shape string) []string {
	s, ok := c.Shapes[shape]
	if !ok {
		return nil
	}
	if s.Required.Flat != nil {
		return append([]string(nil), s.Required.Flat...)
	}
	var out []string
	for _, kind := range []string{"spec", "epic", "item"} {
		for _, f := range s.Required.PerArtifact[kind] {
			out = append(out, kind+"."+f)
		}
	}
	return out
}

// StillRequired returns the required fields for shape that are not yet
// covered by filled. Order matches Required(shape).
func (c Checklist) StillRequired(shape string, filled []string) []string {
	have := make(map[string]struct{}, len(filled))
	for _, f := range filled {
		have[f] = struct{}{}
	}
	var out []string
	for _, r := range c.Required(shape) {
		if _, ok := have[r]; !ok {
			out = append(out, r)
		}
	}
	return out
}
