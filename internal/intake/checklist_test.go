package intake

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestChecklist_EmbeddedDefaultParses(t *testing.T) {
	c, err := LoadChecklist("")
	if err != nil {
		t.Fatalf("load embedded: %v", err)
	}
	if _, ok := c.Shapes["item_only"]; !ok {
		t.Errorf("missing shape item_only")
	}
	if _, ok := c.Shapes["spec_epic_items"]; !ok {
		t.Errorf("missing shape spec_epic_items")
	}

	itemOnly := c.Shapes["item_only"]
	if itemOnly.Required.Flat == nil {
		t.Errorf("item_only.required must be a flat list")
	}
	wantFlat := []string{"title", "intent", "acceptance", "area"}
	if !reflect.DeepEqual(itemOnly.Required.Flat, wantFlat) {
		t.Errorf("item_only.required = %v, want %v", itemOnly.Required.Flat, wantFlat)
	}

	sei := c.Shapes["spec_epic_items"]
	if sei.Required.PerArtifact == nil {
		t.Errorf("spec_epic_items.required must be a per-artifact map")
	}
	for _, kind := range []string{"spec", "epic", "item"} {
		if len(sei.Required.PerArtifact[kind]) == 0 {
			t.Errorf("spec_epic_items.required.%s empty", kind)
		}
	}
}

func TestChecklist_PerRepoOverrideTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	override := []byte(`
shapes:
  item_only:
    description: "custom"
    required: [title, custom_field]
    optional: []
`)
	if err := os.WriteFile(filepath.Join(dir, overrideFilename), override, 0o644); err != nil {
		t.Fatalf("write override: %v", err)
	}
	c, err := LoadChecklist(dir)
	if err != nil {
		t.Fatalf("load with override: %v", err)
	}
	if got := c.Shapes["item_only"].Required.Flat; !reflect.DeepEqual(got, []string{"title", "custom_field"}) {
		t.Errorf("override not applied: required=%v", got)
	}
	if _, ok := c.Shapes["spec_epic_items"]; ok {
		t.Errorf("override should fully replace embedded; spec_epic_items leaked through")
	}
}

func TestChecklist_OverrideMissing_FallsBackToEmbedded(t *testing.T) {
	dir := t.TempDir()
	c, err := LoadChecklist(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := c.Shapes["spec_epic_items"]; !ok {
		t.Errorf("expected embedded default when override absent")
	}
}

func TestChecklist_MissingRequiredDetected_ItemOnly(t *testing.T) {
	c, err := LoadChecklist("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	got := c.StillRequired("item_only", nil)
	sort.Strings(got)
	want := []string{"acceptance", "area", "intent", "title"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nothing filled: still=%v, want %v", got, want)
	}

	got = c.StillRequired("item_only", []string{"title", "intent"})
	sort.Strings(got)
	want = []string{"acceptance", "area"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("title+intent filled: still=%v, want %v", got, want)
	}

	got = c.StillRequired("item_only", []string{"title", "intent", "acceptance", "area"})
	if len(got) != 0 {
		t.Errorf("all filled: still=%v, want empty", got)
	}
}

func TestChecklist_MissingRequiredDetected_SpecEpicItems(t *testing.T) {
	c, err := LoadChecklist("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	all := c.Required("spec_epic_items")
	if len(all) == 0 {
		t.Fatalf("spec_epic_items required list empty")
	}
	for _, name := range []string{"spec.title", "spec.motivation", "epic.title", "item.title", "item.acceptance"} {
		if !contains(all, name) {
			t.Errorf("required missing %s; got %v", name, all)
		}
	}

	got := c.StillRequired("spec_epic_items", []string{"spec.title", "spec.motivation"})
	if contains(got, "spec.title") || contains(got, "spec.motivation") {
		t.Errorf("filled fields appeared in still_required: %v", got)
	}
	if !contains(got, "epic.title") {
		t.Errorf("epic.title should still be required; got %v", got)
	}
}

func TestChecklist_UnknownShape_ReturnsNil(t *testing.T) {
	c, err := LoadChecklist("")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := c.Required("nonexistent"); got != nil {
		t.Errorf("unknown shape should be nil; got %v", got)
	}
	if got := c.StillRequired("nonexistent", []string{"x"}); got != nil {
		t.Errorf("unknown shape should be nil; got %v", got)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
