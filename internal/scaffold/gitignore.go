package scaffold

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var managedIgnoreLines = []string{
	".squad/db-snapshot/",
	".squad/backups/",
	".squad/worktrees/",
}

func EnsureGitignore(repoRoot string) error {
	dest := filepath.Join(repoRoot, ".gitignore")
	cur, err := os.ReadFile(dest)
	if errors.Is(err, fs.ErrNotExist) {
		cur = nil
	} else if err != nil {
		return err
	}
	out := string(cur)
	for _, line := range managedIgnoreLines {
		if hasLine(out, line) {
			continue
		}
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += line + "\n"
	}
	return os.WriteFile(dest, []byte(out), 0o644)
}

func hasLine(content, line string) bool {
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}
