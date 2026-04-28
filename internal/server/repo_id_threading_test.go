package server

import (
	"regexp"
	"strings"
	"testing"
)

// TestSpaThreadsRepoIDIntoDetailFetch pins the workspace-mode contract
// for the SPA's detail-load paths: when the same slug exists in two
// repos, the user must land on the correct detail by virtue of the
// click carrying a repo_id query string. Pre-fix, learnings.js and
// sidebar.js (spec detail) issued bare GETs to /api/learnings/<slug>
// and /api/specs/<name>; the server's resolveItemRepo would silently
// pick the first match, so the user clicking a row in repo A could
// see content from repo B.
//
// The wire contract is structural: the row HTML stashes repo_id in
// data-repo-id, and the detail fetcher reads it and appends
// ?repo_id= when present. Test asserts both pieces are in the
// embedded SPA bytes — same approach as the BUG-043 CSS pin.
func TestSpaThreadsRepoIDIntoDetailFetch(t *testing.T) {
	cases := []struct {
		path     string
		datasetA string // expected `data-repo-id=` site in the row HTML
		fetchA   string // expected `?repo_id=` site in the detail URL
	}{
		{
			path:     "web/learnings.js",
			datasetA: "data-repo-id=",
			fetchA:   "/api/learnings/",
		},
		{
			path:     "web/sidebar.js",
			datasetA: "data-repo-id=",
			fetchA:   "/api/specs/",
		},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			body, err := webFS.ReadFile(c.path)
			if err != nil {
				t.Fatalf("read %s: %v", c.path, err)
			}
			src := string(body)

			if !strings.Contains(src, c.datasetA) {
				t.Errorf("%s: expected list rows to stash repo_id via %q so detail clicks can disambiguate", c.path, c.datasetA)
			}

			// The detail fetch URL must conditionally include
			// ?repo_id=<value>. Match a fragment that names the
			// detail endpoint AND a `?repo_id=` substring within
			// the same JS expression.
			fetchPattern := regexp.MustCompile(regexp.QuoteMeta(c.fetchA) + `[^'"\x60]*` + `\?repo_id=`)
			if !fetchPattern.MatchString(src) {
				// Fall back to matching the simpler shape: any
				// concatenation of the detail path with
				// `?repo_id=` somewhere in the same fetcher.
				detailIdx := strings.Index(src, c.fetchA)
				if detailIdx < 0 {
					t.Fatalf("%s: detail endpoint %q not present in source", c.path, c.fetchA)
				}
				window := src[detailIdx:]
				if len(window) > 600 {
					window = window[:600]
				}
				if !strings.Contains(window, "repo_id") {
					t.Errorf("%s: detail fetcher near %q does not thread repo_id; nearby source:\n%s",
						c.path, c.fetchA, window)
				}
			}
		})
	}
}
