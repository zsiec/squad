package learning

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var stateLineRe = regexp.MustCompile(`(?m)^state:[^\n]*$`)

// Promote moves a learning to a new state directory and rewrites its
// frontmatter state: line atomically (write to .tmp, rename).
func Promote(l Learning, target State) (string, error) {
	if l.State == target {
		return l.Path, nil
	}
	repoRoot, ok := repoRootFromPath(l.Path)
	if !ok {
		return "", fmt.Errorf("cannot derive repo root from %s", l.Path)
	}
	dst := PathFor(repoRoot, l.Kind, target, l.Slug)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	body, err := os.ReadFile(l.Path)
	if err != nil {
		return "", err
	}
	rewritten := stateLineRe.ReplaceAll(body, []byte("state: "+string(target)))
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, rewritten, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Remove(l.Path); err != nil {
		return "", err
	}
	return dst, nil
}

func repoRootFromPath(p string) (string, bool) {
	dir := filepath.Dir(p)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".squad")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
