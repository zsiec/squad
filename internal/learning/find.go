package learning

import (
	"fmt"
	"strings"
)

func ResolveSingle(repoRoot, slug string) (Learning, error) {
	all, err := Walk(repoRoot)
	if err != nil {
		return Learning{}, err
	}
	var hits []Learning
	for _, l := range all {
		if l.Slug == slug {
			hits = append(hits, l)
		}
	}
	switch len(hits) {
	case 0:
		return Learning{}, fmt.Errorf("no learning with slug %q", slug)
	case 1:
		return hits[0], nil
	default:
		var pp []string
		for _, h := range hits {
			pp = append(pp, h.Path)
		}
		return Learning{}, fmt.Errorf("slug %q is ambiguous, matches:\n  %s",
			slug, strings.Join(pp, "\n  "))
	}
}
