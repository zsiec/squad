package specs

import (
	"reflect"
	"testing"
)

func TestParse_FullFrontmatter(t *testing.T) {
	s, err := Parse("testdata/specs/auth-rework.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.Name != "auth-rework" || s.Title != "Auth rework" || s.Motivation == "" {
		t.Errorf("name=%q title=%q motivation empty=%v",
			s.Name, s.Title, s.Motivation == "")
	}
	want := []string{"login redirects to /home", "sessions expire after 30 days"}
	if !reflect.DeepEqual(s.Acceptance, want) {
		t.Errorf("Acceptance=%v want %v", s.Acceptance, want)
	}
}

func TestParse_RejectsPRDFraming(t *testing.T) {
	if _, err := Parse("testdata/specs/with-prd-key.md"); err == nil {
		t.Fatal("expected error when frontmatter contains 'prd:' key")
	}
}

func TestWalk_FindsAllSpecs(t *testing.T) {
	got, err := Walk("testdata")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range got {
		if s.Name == "auth-rework" {
			found = true
		}
	}
	if !found {
		t.Errorf("auth-rework not in %v", got)
	}
}
