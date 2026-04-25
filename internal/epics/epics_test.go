package epics

import (
	"path/filepath"
	"testing"
)

func TestParse_Frontmatter(t *testing.T) {
	e, err := Parse("testdata/epics/auth-login-redirect.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if e.Name != "auth-login-redirect" || e.Spec != "auth-rework" || e.Status != "open" {
		t.Errorf("name=%q spec=%q status=%q", e.Name, e.Spec, e.Status)
	}
}

func TestWalk_FlagsEpicWithMissingSpec(t *testing.T) {
	got, broken, err := Walk("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range got {
		if e.Name == "orphan-epic" {
			t.Error("orphan-epic should be in broken, not got")
		}
	}
	foundOrphan := false
	for _, b := range broken {
		if filepath.Base(b.Path) == "orphan-epic.md" {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Error("orphan-epic should appear in broken list")
	}
}
