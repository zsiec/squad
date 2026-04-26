package items

import (
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

// walkOne enumerates only the immediate .md children of dir. Items must live
// directly under .squad/items/ and .squad/done/ — the lookup helpers in
// claims and the CLI use flat os.ReadDir, so making Walk recursive would
// list items in subdirs that no other code path can claim/move/done. QA
// round-6 H #5 surfaced the divergence; tightening Walk keeps the surface
// consistent.
func walkOne(dir string) ([]Item, []BrokenItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var out []Item
	var broken []BrokenItem
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		it, perr := Parse(path)
		if perr != nil {
			// A file that vanished between ReadDir and Parse is a race,
			// not a malformed item. Swallow it; the next walk picks up
			// the new state.
			if os.IsNotExist(perr) {
				continue
			}
			broken = append(broken, BrokenItem{Path: path, Error: perr.Error()})
			continue
		}
		out = append(out, it)
	}
	return out, broken, nil
}
