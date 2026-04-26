package learning

import (
	"strconv"
	"strings"
)

const NonTrivialNetLines = 10

// NonTrivialDiff reports whether `git diff --numstat` output represents
// enough production-code change to be worth proposing a learning over.
// Test files, docs, and vendored code are excluded.
func NonTrivialDiff(numstat string) bool {
	net := 0
	for _, line := range strings.Split(numstat, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		if isExcludedPath(strings.Join(fields[2:], " ")) {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		net += n
	}
	return net > NonTrivialNetLines
}

func isExcludedPath(p string) bool {
	p = strings.ToLower(p)
	switch {
	case strings.HasPrefix(p, "vendor/"),
		strings.HasPrefix(p, "node_modules/"),
		strings.HasPrefix(p, "docs/"), strings.HasPrefix(p, "doc/"),
		strings.HasPrefix(p, "testdata/"), strings.Contains(p, "/testdata/"):
		return true
	case strings.HasSuffix(p, ".md"), strings.HasSuffix(p, ".txt"),
		strings.HasSuffix(p, "_test.go"),
		strings.HasSuffix(p, ".test.ts"), strings.HasSuffix(p, ".test.tsx"),
		strings.HasSuffix(p, ".spec.ts"), strings.HasSuffix(p, ".spec.tsx"),
		strings.HasSuffix(p, ".spec.js"):
		return true
	}
	return false
}
