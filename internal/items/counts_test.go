package items

import (
	"testing"
	"time"
)

func TestCountAC(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no header", "no AC heading at all", 0},
		{"header no boxes", "## Acceptance criteria\nsome prose, no checkbox\n", 0},
		{"single unchecked", "## Acceptance criteria\n- [ ] one\n", 1},
		{"single checked", "## Acceptance criteria\n- [x] done\n", 1},
		{"mixed checked/unchecked", "## Acceptance criteria\n- [ ] a\n- [x] b\n- [X] c\n- [ ] d\n", 4},
		{"asterisk bullets", "## Acceptance criteria\n* [ ] a\n* [x] b\n", 2},
		{"stops at next header", "## Acceptance criteria\n- [ ] a\n- [ ] b\n\n## Notes\n- [ ] not-counted\n", 2},
		{"checkboxes before AC header ignored", "- [ ] preface\n## Acceptance criteria\n- [ ] a\n", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountAC(tc.body); got != tc.want {
				t.Fatalf("CountAC=%d want %d for body=%q", got, tc.want, tc.body)
			}
		})
	}
}

func TestCountFileRefs(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"no AC header", "no AC heading at all\ninternal/foo/bar.go", 0},
		{"prose has paths but bullets do not",
			"## Acceptance criteria\n- [ ] do the thing\n", 0},
		{"single Go-style file ref",
			"## Acceptance criteria\n- [ ] edit `cmd/squad/claim.go` to wire it\n", 1},
		{"two distinct file refs",
			"## Acceptance criteria\n- [ ] edit cmd/squad/claim.go\n- [ ] edit internal/items/counts.go\n", 2},
		{"duplicate file ref counted once",
			"## Acceptance criteria\n- [ ] edit cmd/squad/claim.go\n- [ ] add tests in cmd/squad/claim.go\n", 1},
		{"bare uppercase markdown doc counts",
			"## Acceptance criteria\n- [ ] update AGENTS.md\n- [ ] update CLAUDE.md\n", 2},
		{"e.g. and i.e. and dotted prose are not paths",
			"## Acceptance criteria\n- [ ] something — e.g. fix it; i.e. just do x.\n- [ ] watch the news\n", 0},
		{"package path counts (no .ext)",
			"## Acceptance criteria\n- [ ] reference internal/items/refine in the docs\n", 1},
		{"file ref outside AC ignored",
			"## Acceptance criteria\n- [ ] no path here\n\n## Notes\n- [ ] internal/foo/bar.go\n", 0},
		{"4 distinct refs across 4 bullets",
			"## Acceptance criteria\n- [ ] cmd/squad/claim.go\n- [ ] internal/items/counts.go\n- [ ] internal/items/walk.go\n- [ ] AGENTS.md\n", 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CountFileRefs(tc.body); got != tc.want {
				t.Fatalf("CountFileRefs=%d want %d for body=%q", got, tc.want, tc.body)
			}
		})
	}
}

func TestCounts_TalliesByCategory(t *testing.T) {
	w, err := Walk("testdata/ready")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
	c := Counts(w, now)
	if c.InProgress != 0 {
		t.Fatalf("in_progress=%d want 0", c.InProgress)
	}
	if c.Ready < 1 {
		t.Fatalf("ready=%d want >=1", c.Ready)
	}
	if c.Blocked != 2 {
		t.Fatalf("blocked=%d want 2", c.Blocked)
	}
}
