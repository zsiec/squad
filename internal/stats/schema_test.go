package stats

import (
	"encoding/json"
	"testing"
)

func TestSnapshotJSONShape(t *testing.T) {
	s := Snapshot{
		SchemaVersion: 1,
		GeneratedAt:   1745596800,
		RepoID:        "github.com/zsiec/squad",
		Window:        Window{Since: 1745510400, Until: 1745596800, Label: "24h"},
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	for _, k := range []string{
		"schema_version", "generated_at", "repo_id", "window",
		"items", "claims", "verification", "learnings", "tokens",
		"by_agent", "by_epic", "series",
	} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing top-level field %q", k)
		}
	}
}

func TestSnapshotZeroValueIsValidJSON(t *testing.T) {
	var s Snapshot
	b, err := json.Marshal(s)
	if err != nil || !json.Valid(b) {
		t.Fatalf("zero value: err=%v body=%s", err, b)
	}
}
