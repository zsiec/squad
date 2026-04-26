package prmark

import (
	"regexp"
	"strings"
)

// ownerRepoRE matches an owner/repo path tail. Owner and repo are each
// constrained to GitHub's character set (alphanumerics, dash, underscore,
// dot — though leading dot is forbidden, we don't separately enforce it
// here). The optional trailing ".git" is consumed by the regex so callers
// don't have to strip it. The final `$` anchors the match — extra path
// components (issues, pulls, ...) are rejected on purpose.
var ownerRepoRE = regexp.MustCompile(`^([A-Za-z0-9._-]+)/([A-Za-z0-9._-]+?)(?:\.git)?/?$`)

// schemePrefixes are matched case-insensitively against the host portion
// (RFC 3986 §3.2.2 — host is case-insensitive). Owner/repo case is
// preserved from the original input since GitHub treats those segments
// as case-sensitive on display.
var schemePrefixes = []string{
	"https://",
	"http://",
	"ssh://git@",
	"git@",
}

// GitHubBaseURL maps a git remote-origin URL to its GitHub web base URL,
// e.g. "https://github.com/owner/repo". Returns "" for any non-GitHub
// origin or malformed input — callers can treat empty as "no links".
//
// Accepts the forms git emits in the wild:
//   - https://github.com/owner/repo[.git]
//   - https://user[:token]@github.com/owner/repo[.git]   (credential helpers rewrite to this)
//   - http://github.com/owner/repo[.git]                 (normalised to https)
//   - git@github.com:owner/repo[.git]
//   - ssh://git@github.com/owner/repo[.git]
//
// Host is matched case-insensitively; owner/repo case is preserved.
func GitHubBaseURL(origin string) string {
	origin = strings.TrimSpace(origin)
	origin = strings.TrimSuffix(origin, "/")
	if origin == "" {
		return ""
	}
	lower := strings.ToLower(origin)

	var rest string
	switch {
	case strings.HasPrefix(lower, "https://"):
		rest = origin[len("https://"):]
	case strings.HasPrefix(lower, "http://"):
		rest = origin[len("http://"):]
	case strings.HasPrefix(lower, "ssh://git@"):
		rest = origin[len("ssh://git@"):]
	case strings.HasPrefix(lower, "git@"):
		rest = origin[len("git@"):]
	default:
		return ""
	}

	// Strip optional userinfo (user[:token]@host) — credential helpers
	// rewrite remotes into this form. Only meaningful before the first
	// '/' or ':' that starts the path.
	if at := strings.IndexByte(rest, '@'); at >= 0 {
		head := rest[:at]
		// userinfo must not contain '/' or ':' that delimits a path —
		// '/' clearly means we already passed the host; ':' is OK
		// (it's the user/pass separator in userinfo).
		if !strings.ContainsAny(head, "/") {
			rest = rest[at+1:]
		}
	}

	// Now rest is host(:port)?[:/]owner/repo[.git]. Both the scp-like
	// "github.com:owner/repo" and URL-style "github.com/owner/repo" land
	// here; collapse the scp colon into a slash.
	if i := strings.IndexAny(rest, "/:"); i >= 0 && rest[i] == ':' {
		rest = rest[:i] + "/" + rest[i+1:]
	}

	host, tail, ok := strings.Cut(rest, "/")
	if !ok {
		return ""
	}
	if !strings.EqualFold(host, "github.com") {
		return ""
	}

	m := ownerRepoRE.FindStringSubmatch(tail)
	if m == nil {
		return ""
	}
	return "https://github.com/" + m[1] + "/" + m[2]
}
