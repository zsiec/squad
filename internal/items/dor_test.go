package items

import "testing"

func TestDoRCheck(t *testing.T) {
	cases := []struct {
		name           string
		it             Item
		wantViolations int
	}{
		{"all good",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] does the thing\n"},
			0},
		{"area is fill-in",
			Item{Title: "investigate the flaky auth test we have", Area: "<fill-in>",
				Body: "## Acceptance criteria\n- [ ] does the thing\n"},
			1},
		{"area empty",
			Item{Title: "investigate the flaky auth test we have", Area: "",
				Body: "## Acceptance criteria\n- [ ] does the thing\n"},
			1},
		{"no acceptance criteria checkbox",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\nsome prose with no checkbox\n"},
			1},
		{"no AC heading at all",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Notes\nhi\n"},
			1},
		{"short title with empty problem",
			Item{Title: "fix bug", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] x\n## Problem\n\n"},
			1},
		{"short title with non-empty problem",
			Item{Title: "fix bug", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] x\n## Problem\nsomething is wrong\n"},
			0},
		{"5-word title boundary (not enough — rule says >5 means 6+)",
			Item{Title: "one two three four five", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] x\n"},
			1},
		{"6-word title boundary (enough)",
			Item{Title: "one two three four five six", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] x\n"},
			0},
		{"checked box also counts",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [x] already done\n"},
			0},
		{"AC heading with checkboxes after a sub-heading boundary",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] x\n\n## Notes\n- [ ] y\n"},
			0},
		{"checkbox AFTER another section is NOT counted",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\nno checkbox here\n\n## Notes\n- [ ] y\n"},
			1},
		{"all three rules fail",
			Item{Title: "fix it", Area: "<fill-in>",
				Body: "no AC heading at all"},
			3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DoRCheck(c.it)
			if len(got) != c.wantViolations {
				t.Fatalf("got %d violations, want %d: %+v", len(got), c.wantViolations, got)
			}
		})
	}
}
