package githubactions

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOnPRMergeYAMLParses(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(".", "on-pr-merge.yml"))
	if err != nil {
		t.Fatalf("read yaml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parse yaml: %v", err)
	}

	on, ok := doc["on"].(map[string]any)
	if !ok {
		t.Fatalf("missing or malformed `on` block; doc=%v", doc)
	}
	pr, ok := on["pull_request"].(map[string]any)
	if !ok {
		t.Fatalf("missing pull_request trigger")
	}
	types, ok := pr["types"].([]any)
	if !ok || len(types) == 0 || types[0] != "closed" {
		t.Fatalf("pull_request.types must include 'closed'; got %v", types)
	}

	jobs, ok := doc["jobs"].(map[string]any)
	if !ok || len(jobs) == 0 {
		t.Fatalf("missing jobs block")
	}

	conc, ok := doc["concurrency"].(map[string]any)
	if !ok {
		t.Fatalf("workflow must declare concurrency")
	}
	if conc["group"] != "squad-pr-close" {
		t.Fatalf("concurrency group must be squad-pr-close, got %v", conc["group"])
	}
	if conc["cancel-in-progress"] != false {
		t.Fatalf("cancel-in-progress must be false to avoid losing archive work")
	}
}
