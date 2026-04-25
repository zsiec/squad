package claims

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/items"
)

func preflightBlockers(itemsDir, doneDir, itemID string) error {
	if strings.TrimSpace(itemID) == "" {
		return ErrItemNotFound
	}
	itemPath, err := findItemFile(itemsDir, itemID)
	if err != nil {
		// Not in items/. If it's in done/, surface that specifically;
		// otherwise the id has no backing item file at all.
		if blockerInDoneDir(doneDir, itemID) {
			return ErrItemAlreadyDone
		}
		return ErrItemNotFound
	}
	it, err := items.Parse(itemPath)
	if err != nil {
		return ErrItemNotFound
	}
	if len(it.BlockedBy) == 0 {
		return nil
	}
	var unresolved []string
	for _, b := range it.BlockedBy {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		if !blockerInDoneDir(doneDir, b) {
			unresolved = append(unresolved, b)
		}
	}
	if len(unresolved) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s is blocked by %s — these must be done first", ErrBlockedByOpen, itemID, strings.Join(unresolved, ", "))
}

func findItemFile(dir, itemID string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		it, err := items.Parse(path)
		if err != nil {
			continue
		}
		if it.ID == itemID {
			return path, nil
		}
	}
	return "", fmt.Errorf("item %s not found in %s", itemID, dir)
}

func conflictsWithPaths(itemsDir, itemID string) ([]string, error) {
	p, err := findItemFile(itemsDir, itemID)
	if err != nil {
		return nil, err
	}
	it, err := items.Parse(p)
	if err != nil {
		return nil, err
	}
	return it.ConflictsWith, nil
}

func blockerInDoneDir(doneDir, blockerID string) bool {
	entries, err := os.ReadDir(doneDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(doneDir, e.Name())
		it, err := items.Parse(path)
		if err != nil {
			continue
		}
		if it.ID == blockerID {
			return true
		}
	}
	return false
}
