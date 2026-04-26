// Package installer materializes the embedded plugin tree to
// ~/.claude/plugins/squad/, merges squad's MCP server entry into
// ~/.claude/settings.json, and registers the squad-managed Claude Code
// hooks. All of this is reversible via the Uninstall and Unmerge helpers.
package installer

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func Install(src fs.FS, dst string) error {
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}

	stage, err := os.MkdirTemp(parent, ".squad-install-*")
	if err != nil {
		return err
	}
	cleanupStage := stage
	defer func() {
		if cleanupStage != "" {
			os.RemoveAll(cleanupStage)
		}
	}()

	if err := copyFS(src, stage); err != nil {
		return err
	}

	if err := os.RemoveAll(dst); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(stage, dst); err != nil {
		return err
	}
	cleanupStage = ""
	return nil
}

func Uninstall(dst string) error {
	err := os.RemoveAll(dst)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func copyFS(src fs.FS, dst string) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		target := filepath.Join(dst, path)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(src, path, target)
	})
}

func copyFile(src fs.FS, srcPath, dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}
	in, err := src.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
