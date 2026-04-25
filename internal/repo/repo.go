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
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", ErrNotInitialized
		}
		dir = parent
	}
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
	data, err := os.ReadFile(filepath.Join(rootPath, ".git", "config"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read git config: %w", err)
	}
	return parseOriginURL(string(data)), nil
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
