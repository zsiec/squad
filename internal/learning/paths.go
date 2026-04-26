package learning

import (
	"path/filepath"
	"strings"
)

func LearningsRoot(repoRoot string) string {
	return filepath.Join(repoRoot, ".squad", "learnings")
}

func PathFor(repoRoot string, k Kind, s State, slug string) string {
	return filepath.Join(LearningsRoot(repoRoot), k.Dir(), string(s), slug+".md")
}

func DirFor(repoRoot string, k Kind, s State) string {
	return filepath.Join(LearningsRoot(repoRoot), k.Dir(), string(s))
}

func ParsePath(repoRoot, p string) (Kind, State, string, bool) {
	rel, err := filepath.Rel(LearningsRoot(repoRoot), p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", "", false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) != 3 || !strings.HasSuffix(parts[2], ".md") {
		return "", "", "", false
	}
	var k Kind
	switch parts[0] {
	case "gotchas":
		k = KindGotcha
	case "patterns":
		k = KindPattern
	case "dead-ends":
		k = KindDeadEnd
	default:
		return "", "", "", false
	}
	st, err := ParseState(parts[1])
	if err != nil {
		return "", "", "", false
	}
	return k, st, strings.TrimSuffix(parts[2], ".md"), true
}
