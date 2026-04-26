package config

import "github.com/bmatcuk/doublestar/v4"

// AnyGlobMatches reports whether path matches any of the doublestar globs.
// Used by the touch policy to decide whether a conflicting edit lands on a
// red-flag path (go.mod, **/*.lock, schema files) and should be denied
// outright instead of merely surfaced for user confirmation.
func AnyGlobMatches(globs []string, path string) bool {
	for _, g := range globs {
		if matched, err := doublestar.PathMatch(g, path); err == nil && matched {
			return true
		}
	}
	return false
}
