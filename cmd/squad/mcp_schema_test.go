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

// schemaEnum returns the enum slice declared on the named property, or nil
// if no enum constraint is present. Used by the list_items drift guards.
func schemaEnum(t *testing.T, schema, prop string) []string {
	t.Helper()
	var s struct {
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(schema), &s); err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	p, ok := s.Properties[prop]
	if !ok {
		t.Fatalf("schema has no property %q", prop)
	}
	return p.Enum
}

// TestListItemsStatusEnumMatchesRuntime guards the squad_list_items input
// schema's `status` enum against drifting away from the canonical lifecycle
// values items.go writes to disk (`captured`, `open`, `in_progress`,
// `blocked`, `done`). The previous enum claimed `ready`/`review` which the
// item lifecycle has never produced.
func TestListItemsStatusEnumMatchesRuntime(t *testing.T) {
	got := schemaEnum(t, schemaListItems, "status")
	want := map[string]bool{
		"captured":    true,
		"open":        true,
		"in_progress": true,
		"blocked":     true,
		"done":        true,
	}
	if len(got) != len(want) {
		t.Fatalf("status enum size = %d %v, want %d %v", len(got), got, len(want), keysOf(want))
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("status enum has %q which the items lifecycle does not produce; want one of %v", v, keysOf(want))
		}
	}
}

// TestListItemsTypeEnumIsOpenEnded asserts the squad_list_items input schema's
// `type` field has no enum constraint. Type vocabulary is config-driven via
// id_prefixes — pinning an enum freezes a stale list and breaks any user with
// a non-default prefix set.
func TestListItemsTypeEnumIsOpenEnded(t *testing.T) {
	got := schemaEnum(t, schemaListItems, "type")
	if len(got) != 0 {
		t.Fatalf("type field must not declare an enum (config-driven via id_prefixes); got %v", got)
	}
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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
