package items

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
)

// filenameIDRe pulls a PREFIX-NUMBER from the start of the basename of an
// item file. Used so files that Parse rejected (broken YAML) still reserve
// their slot in the ID sequence — otherwise NextID would re-issue the same
// id and the next `squad new` would silently overwrite or duplicate.
var filenameIDRe = regexp.MustCompile(`^([A-Z][A-Z0-9]*)-(\d+)`)

func NextID(prefix string, w WalkResult) (string, error) {
	max := 0
	for _, src := range [][]Item{w.Active, w.Done} {
		for _, it := range src {
			p, n, err := ParseID(it.ID)
			if err != nil || p != prefix {
				continue
			}
			if n > max {
				max = n
			}
		}
	}
	for _, b := range w.Broken {
		m := filenameIDRe.FindStringSubmatch(filepath.Base(b.Path))
		if m == nil || m[1] != prefix {
			continue
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s-%03d", prefix, max+1), nil
}
