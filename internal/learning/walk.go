package learning

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

func Walk(repoRoot string) ([]Learning, error) {
	root := LearningsRoot(repoRoot)
	var out []Learning
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if strings.Contains(err.Error(), "no such file") {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		l, perr := Parse(p)
		if perr != nil {
			return nil
		}
		out = append(out, l)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
