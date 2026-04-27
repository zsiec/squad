package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrNotInitialized = errors.New("no .squad/config.yaml found in CWD or any parent — run `squad init` first")

func Discover(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".squad", "config.yaml")); err == nil {
			return canonicalize(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInitialized
		}
		dir = parent
	}
}

// canonicalize resolves symlinks and returns an absolute path so repo_id
// derivation is deterministic across callers. On macOS, /tmp is a symlink
// to /private/tmp; without resolution, init's `git rev-parse --show-toplevel`
// (which returns the literal CWD) and the rest of the binary's `os.Getwd()`
// (which resolves symlinks) yield different repo_ids for the same repo.
func canonicalize(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	return p
}

func DeriveRepoID(remoteURL, rootPath string) string {
	if remoteURL == "" {
		sum := sha256.Sum256([]byte("path:" + rootPath))
		return hex.EncodeToString(sum[:8])
	}
	sum := sha256.Sum256([]byte(remoteURL))
	return hex.EncodeToString(sum[:8])
}

func ReadRemoteURL(rootPath string) (string, error) {
	cfgPath, err := gitConfigPath(rootPath)
	if err != nil {
		return "", fmt.Errorf("read git config: %w", err)
	}
	if cfgPath == "" {
		return "", nil
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read git config: %w", err)
	}
	return parseOriginURL(string(data)), nil
}

// gitConfigPath returns the absolute path to the git config file for a
// checkout rooted at rootPath. In a regular checkout `<rootPath>/.git`
// is a directory and config sits inside it. In a git worktree `.git`
// is a regular file containing `gitdir: <path-to-shared-git-dir>/worktrees/<name>`,
// and the shared config lives in the main git dir, not the worktree's
// gitdir. Returns "" with no error when no .git entry exists at all,
// matching the silent fallback ReadRemoteURL had before.
func gitConfigPath(rootPath string) (string, error) {
	dotGit := filepath.Join(rootPath, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	if info.IsDir() {
		return filepath.Join(dotGit, "config"), nil
	}
	raw, err := os.ReadFile(dotGit)
	if err != nil {
		return "", err
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(raw)), "gitdir:"))
	if gitdir == "" {
		return "", fmt.Errorf("malformed .git pointer in %s: missing gitdir", dotGit)
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(rootPath, gitdir)
	}
	// Modern git writes a `commondir` file in the worktree's gitdir
	// pointing back at the shared git dir (relative to gitdir).
	if cd, err := os.ReadFile(filepath.Join(gitdir, "commondir")); err == nil {
		common := strings.TrimSpace(string(cd))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
		return filepath.Join(common, "config"), nil
	}
	// Fallback for older git or hand-crafted layouts: the standard
	// shape is <main-git>/worktrees/<name>, so two levels up from
	// gitdir is the shared git dir.
	return filepath.Join(filepath.Dir(filepath.Dir(gitdir)), "config"), nil
}

func parseOriginURL(gitConfig string) string {
	inOrigin := false
	for _, line := range strings.Split(gitConfig, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if t == `[remote "origin"]` {
			inOrigin = true
			continue
		}
		if strings.HasPrefix(t, "[") {
			inOrigin = false
			continue
		}
		if !inOrigin {
			continue
		}
		if idx := strings.Index(t, "="); idx > 0 {
			k := strings.TrimSpace(t[:idx])
			v := strings.TrimSpace(t[idx+1:])
			if k == "url" {
				return v
			}
		}
	}
	return ""
}
