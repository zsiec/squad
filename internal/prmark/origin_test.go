package prmark

import "testing"

func TestGitHubBaseURL(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		want   string
	}{
		{"empty", "", ""},
		{"https with .git", "https://github.com/zsiec/squad.git", "https://github.com/zsiec/squad"},
		{"https no .git", "https://github.com/zsiec/squad", "https://github.com/zsiec/squad"},
		{"https with trailing slash", "https://github.com/zsiec/squad/", "https://github.com/zsiec/squad"},
		{"http normalised to https", "http://github.com/zsiec/squad.git", "https://github.com/zsiec/squad"},
		{"ssh form with .git", "git@github.com:zsiec/squad.git", "https://github.com/zsiec/squad"},
		{"ssh form no .git", "git@github.com:zsiec/squad", "https://github.com/zsiec/squad"},
		{"ssh:// scheme form", "ssh://git@github.com/zsiec/squad.git", "https://github.com/zsiec/squad"},
		{"owner with hyphen and digits", "git@github.com:org-9/repo-name.git", "https://github.com/org-9/repo-name"},
		{"repo with underscore and dot", "https://github.com/o/repo.name_x.git", "https://github.com/o/repo.name_x"},
		{"non-github gitlab", "https://gitlab.com/zsiec/squad.git", ""},
		{"non-github bitbucket ssh", "git@bitbucket.org:o/r.git", ""},
		{"malformed", "not-a-url", ""},
		{"github but no path", "https://github.com/", ""},
		{"github with only owner", "https://github.com/zsiec", ""},
		{"github with extra path components rejects", "https://github.com/zsiec/squad/issues/1", ""},
		{"unicode owner rejected", "https://github.com/öwner/repo", ""},
		{"shell injection rejected", "git@github.com:o/r;rm -rf /.git", ""},
		{"whitespace stripped", "  https://github.com/zsiec/squad.git  \n", "https://github.com/zsiec/squad"},
		{"userinfo https with token", "https://user:ghp_token@github.com/zsiec/squad.git", "https://github.com/zsiec/squad"},
		{"userinfo https with bare user", "https://zsiec@github.com/zsiec/squad.git", "https://github.com/zsiec/squad"},
		{"host case-insensitive https", "https://GITHUB.com/zsiec/Squad.git", "https://github.com/zsiec/Squad"},
		{"host case-insensitive ssh", "git@GITHUB.com:zsiec/Squad.git", "https://github.com/zsiec/Squad"},
		{"owner/repo case preserved", "git@github.com:MyOrg/MyRepo.git", "https://github.com/MyOrg/MyRepo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := GitHubBaseURL(tc.origin)
			if got != tc.want {
				t.Fatalf("GitHubBaseURL(%q) = %q, want %q", tc.origin, got, tc.want)
			}
		})
	}
}
