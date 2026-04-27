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
