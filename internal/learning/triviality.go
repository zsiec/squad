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
		if isExcludedPath(renamedNewPath(strings.Join(fields[2:], " "))) {
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

// renamedNewPath collapses `git diff --numstat` rename forms to the new path.
// Brace form:  "{old => new}/rest"  → "new/rest" (or "rest" if new is empty).
// Arrow form:  "old => new"          → "new".
// Plain path:  unchanged.
func renamedNewPath(p string) string {
	if !strings.Contains(p, " => ") {
		return p
	}
	if open := strings.LastIndex(p, "{"); open >= 0 {
		if close := strings.Index(p[open:], "}"); close > 0 {
			inside := p[open+1 : open+close]
			if i := strings.Index(inside, " => "); i >= 0 {
				newPart := strings.TrimSpace(inside[i+len(" => "):])
				out := p[:open] + newPart + p[open+close+1:]
				return strings.TrimPrefix(out, "/")
			}
		}
	}
	if i := strings.Index(p, " => "); i >= 0 {
		return strings.TrimSpace(p[i+len(" => "):])
	}
	return p
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
