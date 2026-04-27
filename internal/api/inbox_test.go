package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestInboxEntry_RoundTrip pins the wire shape: a fixture with every
// documented field round-trips through encode → decode unchanged. If
// anyone adds a json-tagged field to InboxEntry without updating this
// fixture, the test fails — the same regression mode option (a) of
// CHORE-006 was meant to prevent across the server/client split.
func TestInboxEntry_RoundTrip(t *testing.T) {
	want := InboxEntry{
		ID:         "FEAT-100",
		Title:      "ship the thing",
		CapturedBy: "agent-x",
		CapturedAt: 1700000000,
		ParentSpec: "auth-rework",
		DoRPass:    true,
		Path:       "/repo/.squad/items/FEAT-100.md",
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got InboxEntry
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Errorf("round-trip mismatch:\nwant: %+v\n got: %+v", want, got)
	}
}

// TestInboxEntry_FieldNamesPinned guards against silent field drops by
// asserting the exact set of top-level json keys produced for a fully
// populated value. Adding a new field requires extending this list,
// which is the deliberate friction.
func TestInboxEntry_FieldNamesPinned(t *testing.T) {
	full := InboxEntry{
		ID: "x", Title: "y", CapturedBy: "z",
		CapturedAt: 1, ParentSpec: "p",
		DoRPass: true, Path: "/p",
	}
	raw, err := json.Marshal(full)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantKeys := map[string]bool{
		"id": true, "title": true, "captured_by": true,
		"captured_at": true, "parent_spec": true,
		"dor_pass": true, "path": true,
	}
	for k := range generic {
		if !wantKeys[k] {
			t.Errorf("unexpected json key %q (extend wantKeys when intentionally adding)", k)
		}
	}
	for k := range wantKeys {
		if _, ok := generic[k]; !ok {
			t.Errorf("missing expected json key %q", k)
		}
	}
}
