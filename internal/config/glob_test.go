package config

import "testing"

func TestAnyGlobMatches(t *testing.T) {
	cases := []struct {
		name  string
		globs []string
		path  string
		want  bool
	}{
		{"empty list", nil, "go.mod", false},
		{"exact match", []string{"go.mod"}, "go.mod", true},
		{"exact non-match", []string{"go.mod"}, "go.sum", false},
		{"single star segment", []string{"*.lock"}, "yarn.lock", true},
		{"single star not crossing slash", []string{"*.lock"}, "vendor/yarn.lock", false},
		{"double star nested", []string{"**/*.lock"}, "vendor/foo/yarn.lock", true},
		{"double star at root", []string{"**/*.lock"}, "yarn.lock", true},
		{"first of many", []string{"go.mod", "**/*.lock"}, "go.mod", true},
		{"second of many", []string{"go.mod", "**/*.lock"}, "vendor/yarn.lock", true},
		{"none of many", []string{"go.mod", "**/*.lock"}, "internal/foo.go", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AnyGlobMatches(tc.globs, tc.path); got != tc.want {
				t.Fatalf("AnyGlobMatches(%v, %q) = %v, want %v", tc.globs, tc.path, got, tc.want)
			}
		})
	}
}
