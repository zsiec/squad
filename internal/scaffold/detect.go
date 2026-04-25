package scaffold

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type RepoInfo struct {
	GitRoot         string
	Remote          string
	PrimaryLanguage string
	ProjectBasename string
}

var ErrNotGitRepo = errors.New("not a git repository")

func DetectRepo(startDir string) (RepoInfo, error) {
	root, err := findGitRoot(startDir)
	if err != nil {
		return RepoInfo{}, err
	}
	return RepoInfo{
		GitRoot:         root,
		Remote:          readRemote(root),
		PrimaryLanguage: detectLanguage(root),
		ProjectBasename: filepath.Base(root),
	}, nil
}

func findGitRoot(start string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = start
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNotGitRepo, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func readRemote(gitRoot string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectLanguage(root string) string {
	checks := []struct {
		filename string
		lang     string
	}{
		{"go.mod", "go"},
		{"package.json", "node"},
		{"Cargo.toml", "rust"},
		{"pyproject.toml", "python"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(root, c.filename)); err == nil {
			return c.lang
		}
	}
	return ""
}
