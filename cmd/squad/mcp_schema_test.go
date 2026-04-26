package main

import (
	"encoding/json"
	"sort"
	"testing"
)

// schemaProperties returns the top-level property names declared in a
// JSON-Schema string. Used by drift guards below.
func schemaProperties(t *testing.T, schema string) []string {
	t.Helper()
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal([]byte(schema), &s); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	out := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestReleaseSchemaMatchesHandler guards against properties accumulating in
// schemaRelease that the squad_release handler never reads. The handler's
// local args struct accepts {item_id, outcome, agent_id}; the schema must
// match. (Lifted ReleaseArgs has no Reason field — declaring "reason" in
// the schema would mean callers see it silently dropped.)
func TestReleaseSchemaMatchesHandler(t *testing.T) {
	got := schemaProperties(t, schemaRelease)
	want := []string{"agent_id", "item_id", "outcome"}
	if !equalStrings(got, want) {
		t.Fatalf("schemaRelease properties = %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
