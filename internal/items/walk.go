package items

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type WalkResult struct {
	Active []Item
	Done   []Item
}

func Walk(squadDir string) (WalkResult, error) {
	active, err := walkOne(filepath.Join(squadDir, "items"))
	if err != nil {
		return WalkResult{}, err
	}
	done, err := walkOne(filepath.Join(squadDir, "done"))
	if err != nil {
		return WalkResult{}, err
	}
	return WalkResult{Active: active, Done: done}, nil
}

func walkOne(dir string) ([]Item, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil
	}
	var out []Item
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		it, perr := Parse(path)
		if perr != nil {
			return nil
		}
		out = append(out, it)
		return nil
	})
	return out, err
}
