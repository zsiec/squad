package items

import (
	"os"
	"path/filepath"
	"testing"
)

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
		{"AC is the unmodified squad-new template placeholders",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] Specific, testable thing 1\n- [ ] Specific, testable thing 2\n"},
			1},
		{"AC has one placeholder swapped for a real criterion",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] Specific, testable thing 1\n- [ ] real criterion that means something\n"},
			0},
		{"AC has only one placeholder remaining (other deleted)",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] Specific, testable thing 2\n"},
			1},
		{"AC has placeholders AND template prose in Problem/Context — only AC counts",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Problem\nWhat is wrong / what doesn't exist. 1–3 sentences.\n\n## Context\nWhy this matters.\n\n## Acceptance criteria\n- [ ] real, testable thing\n"},
			0},
		{"AC has near-miss text — exact match required",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [ ] Specific testable thing 1\n- [ ] Specific testable thing 2\n"},
			0},
		{"AC placeholders with checked boxes still trip the rule",
			Item{Title: "investigate the flaky auth test we have", Area: "auth",
				Body: "## Acceptance criteria\n- [x] Specific, testable thing 1\n- [ ] Specific, testable thing 2\n"},
			1},
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

// A freshly-created `squad new` item must trip the template-not-placeholder
// rule end-to-end: this proves the rule's sentinel list and the stub template
// are sourced from the same constants and cannot drift.
func TestDoRCheck_FreshSquadNewItemFailsTemplateRule(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := NewWithOptions(dir, "BUG", "investigate the flaky auth test we have", Options{Area: "auth"})
	if err != nil {
		t.Fatal(err)
	}
	it, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	violations := DoRCheck(it)
	var found bool
	for _, v := range violations {
		if v.Rule == "template-not-placeholder" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected template-not-placeholder violation on fresh squad-new body; got %+v", violations)
	}
}
