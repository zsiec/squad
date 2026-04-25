package prmark

import "testing"

func TestExtract(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", "", ""},
		{"no marker", "Plain PR description.\n\nFixes the thing.", ""},
		{"marker alone", "<!-- squad-item: BUG-001 -->", "BUG-001"},
		{"marker with surrounding text", "## Summary\n\nFixed it.\n\n<!-- squad-item: FEAT-042 -->\n", "FEAT-042"},
		{"marker with extra spaces", "<!--   squad-item:    TASK-7   -->", "TASK-7"},
		{"two markers — first wins", "<!-- squad-item: BUG-1 --> and later <!-- squad-item: BUG-2 -->", "BUG-1"},
		{"missing closing", "<!-- squad-item: BUG-3", ""},
		{"wrong key", "<!-- squad item: BUG-4 -->", ""},
		{"lowercase id rejected (case-strict)", "<!-- squad-item: bug-9 -->", ""},
		{"non-prefix garbage rejected", "<!-- squad-item: <script> -->", ""},
		{"shell-injection-shaped rejected", "<!-- squad-item: BUG-1; rm -rf / -->", ""},
		{"trailing space ok", "<!-- squad-item: BUG-1 -->", "BUG-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Extract(tc.body)
			if got != tc.want {
				t.Fatalf("Extract(%q) = %q, want %q", tc.body, got, tc.want)
			}
		})
	}
}

func TestFormat(t *testing.T) {
	got := Format("BUG-001")
	want := "<!-- squad-item: BUG-001 -->"
	if got != want {
		t.Fatalf("Format = %q, want %q", got, want)
	}
}
