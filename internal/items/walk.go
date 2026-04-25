package items

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BrokenItem records a file that walked but failed to parse. Surfacing these
// (rather than silently dropping) lets `squad doctor` warn the user about
// CRLF/BOM/malformed-YAML files that would otherwise be invisible.
type BrokenItem struct {
	Path  string
	Error string
}

type WalkResult struct {
	Active []Item
	Done   []Item
	// Broken lists files in items/ or done/ that filepath.WalkDir reached but
	// items.Parse rejected. Callers (doctor, mirror) can decide how to react;
	// next/status/workspace status simply ignore broken files in their counts.
	Broken []BrokenItem
}

func Walk(squadDir string) (WalkResult, error) {
	active, brokenA, err := walkOne(filepath.Join(squadDir, "items"))
	if err != nil {
		return WalkResult{}, err
	}
	done, brokenD, err := walkOne(filepath.Join(squadDir, "done"))
	if err != nil {
		return WalkResult{}, err
	}
	return WalkResult{Active: active, Done: done, Broken: append(brokenA, brokenD...)}, nil
}

func walkOne(dir string) ([]Item, []BrokenItem, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil, nil
	}
	var out []Item
	var broken []BrokenItem
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		it, perr := Parse(path)
		if perr != nil {
			broken = append(broken, BrokenItem{Path: path, Error: perr.Error()})
			return nil
		}
		out = append(out, it)
		return nil
	})
	return out, broken, err
}
