package items

import "fmt"

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
	return fmt.Sprintf("%s-%03d", prefix, max+1), nil
}
