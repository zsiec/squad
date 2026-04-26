package learning

import (
	"path/filepath"
	"testing"
)

func TestPathFor(t *testing.T) {
	cases := []struct {
		k    Kind
		s    State
		slug string
		want string
	}{
		{KindGotcha, StateProposed, "sqlite-busy", "/repo/.squad/learnings/gotchas/proposed/sqlite-busy.md"},
		{KindPattern, StateApproved, "boot-context", "/repo/.squad/learnings/patterns/approved/boot-context.md"},
		{KindDeadEnd, StateRejected, "fork-isolate", "/repo/.squad/learnings/dead-ends/rejected/fork-isolate.md"},
	}
	for _, c := range cases {
		got := PathFor("/repo", c.k, c.s, c.slug)
		if got != filepath.FromSlash(c.want) {
			t.Errorf("PathFor(%v,%v,%q) = %q, want %q", c.k, c.s, c.slug, got, c.want)
		}
	}
}

func TestParsePath_Roundtrip(t *testing.T) {
	in := filepath.Join("/repo", ".squad", "learnings", "patterns", "approved", "boot-context.md")
	k, s, slug, ok := ParsePath("/repo", in)
	if !ok || k != KindPattern || s != StateApproved || slug != "boot-context" {
		t.Errorf("got (%v,%v,%q,%v)", k, s, slug, ok)
	}
}

func TestParsePath_OutsideTreeFails(t *testing.T) {
	if _, _, _, ok := ParsePath("/repo", "/elsewhere/x.md"); ok {
		t.Errorf("expected ok=false for path outside .squad/learnings")
	}
}
