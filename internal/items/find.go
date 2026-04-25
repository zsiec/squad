package items

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var ErrItemNotFound = errors.New("item not found")

// FindByID locates the markdown file for itemID by scanning .squad/items/ then
// .squad/done/ under squadDir. Returns the absolute path and a flag indicating
// whether it lives in done/. Returns ErrItemNotFound if no match.
func FindByID(squadDir, itemID string) (path string, inDone bool, err error) {
	for _, sub := range []string{"items", "done"} {
		dir := filepath.Join(squadDir, sub)
		entries, derr := os.ReadDir(dir)
		if derr != nil {
			if errors.Is(derr, fs.ErrNotExist) {
				continue
			}
			return "", false, derr
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if e.Name() == itemID+".md" || strings.HasPrefix(e.Name(), itemID+"-") {
				return filepath.Join(dir, e.Name()), sub == "done", nil
			}
		}
	}
	return "", false, ErrItemNotFound
}
