// Package analyze produces a deterministic stream-decomposition for an
// epic's items. Same input → same output. No model invocation.
package analyze

import (
	"sort"

	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
)

type Stream struct {
	FileGlobs []string
	ItemIDs   []string
}

type DepEdge struct {
	From string
	To   string
}

type Analysis struct {
	Epic              string
	Spec              string
	Streams           []Stream
	Deps              []DepEdge
	HasCycle          bool
	ParallelismFactor float64
}

func Run(e epics.Epic, its []items.Item) Analysis {
	a := Analysis{Epic: e.Name, Spec: e.Spec}
	if len(its) == 0 {
		return a
	}

	parent := make(map[string]string, len(its))
	var find func(string) string
	find = func(x string) string {
		if parent[x] == x {
			return x
		}
		parent[x] = find(parent[x])
		return parent[x]
	}
	union := func(x, y string) {
		if rx, ry := find(x), find(y); rx != ry {
			parent[rx] = ry
		}
	}
	for _, it := range its {
		parent[it.ID] = it.ID
	}
	pathOwners := map[string]string{}
	for _, it := range its {
		for _, p := range it.ConflictsWith {
			if owner, ok := pathOwners[p]; ok {
				union(it.ID, owner)
			} else {
				pathOwners[p] = it.ID
			}
		}
	}

	groups := map[string][]string{}
	for _, it := range its {
		groups[find(it.ID)] = append(groups[find(it.ID)], it.ID)
	}
	idToItem := map[string]items.Item{}
	for _, it := range its {
		idToItem[it.ID] = it
	}
	roots := make([]string, 0, len(groups))
	for r := range groups {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	for _, r := range roots {
		ids := groups[r]
		sort.Strings(ids)
		globSet := map[string]struct{}{}
		for _, id := range ids {
			for _, p := range idToItem[id].ConflictsWith {
				globSet[p] = struct{}{}
			}
		}
		globs := make([]string, 0, len(globSet))
		for g := range globSet {
			globs = append(globs, g)
		}
		sort.Strings(globs)
		a.Streams = append(a.Streams, Stream{FileGlobs: globs, ItemIDs: ids})
	}

	known := map[string]bool{}
	for _, it := range its {
		known[it.ID] = true
	}
	for _, it := range its {
		for _, d := range it.DependsOn {
			if known[d] {
				a.Deps = append(a.Deps, DepEdge{From: d, To: it.ID})
			}
		}
	}
	a.HasCycle = detectCycle(its)
	a.ParallelismFactor = float64(len(its)) / float64(len(a.Streams))
	if a.ParallelismFactor < 1 {
		a.ParallelismFactor = 1
	}
	return a
}

func detectCycle(its []items.Item) bool {
	graph := map[string][]string{}
	for _, it := range its {
		graph[it.ID] = nil
	}
	for _, it := range its {
		for _, d := range it.DependsOn {
			if _, ok := graph[d]; ok {
				graph[it.ID] = append(graph[it.ID], d)
			}
		}
	}
	const (
		white, gray, black = 0, 1, 2
	)
	color := map[string]int{}
	var dfs func(string) bool
	dfs = func(n string) bool {
		color[n] = gray
		for _, m := range graph[n] {
			switch color[m] {
			case gray:
				return true
			case white:
				if dfs(m) {
					return true
				}
			}
		}
		color[n] = black
		return false
	}
	for n := range graph {
		if color[n] == white && dfs(n) {
			return true
		}
	}
	return false
}
