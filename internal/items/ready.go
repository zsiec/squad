package items

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var priorityScore = map[string]int{"P0": 0, "P1": 1, "P2": 2, "P3": 3}

func Ready(w WalkResult, now time.Time) []Item {
	statusByID := map[string]string{}
	for _, it := range w.Active {
		statusByID[it.ID] = it.Status
	}
	for _, it := range w.Done {
		statusByID[it.ID] = "done"
	}
	var out []Item
	for _, it := range w.Active {
		if it.Status == "done" || it.Status == "blocked" {
			continue
		}
		if gatedUntil(it, now) {
			continue
		}
		if hasOpenBlocker(it, statusByID) {
			continue
		}
		if hasUnsatisfiedDep(it, statusByID) {
			continue
		}
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := scoreOf(out[i].Priority), scoreOf(out[j].Priority)
		if pi != pj {
			return pi < pj
		}
		ei, _ := EstimateHours(out[i].Estimate)
		ej, _ := EstimateHours(out[j].Estimate)
		return ei < ej
	})
	return out
}

func scoreOf(p string) int {
	if s, ok := priorityScore[p]; ok {
		return s
	}
	return 99
}

func hasOpenBlocker(it Item, statusByID map[string]string) bool {
	for _, bid := range it.BlockedBy {
		if bid == "" {
			continue
		}
		st, known := statusByID[bid]
		if !known || st == "done" {
			continue
		}
		return true
	}
	return false
}

// Unlike blocked-by, depends_on treats an unknown id (no row in statusByID)
// as unsatisfied. R3 epics are planned ahead, so referencing a not-yet-created
// item is a "wait" signal, not a "skip" signal.
func hasUnsatisfiedDep(it Item, statusByID map[string]string) bool {
	for _, d := range it.DependsOn {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if statusByID[d] != "done" {
			return true
		}
	}
	return false
}

func gatedUntil(it Item, now time.Time) bool {
	if it.NotBefore == "" {
		return false
	}
	t, err := time.Parse("2006-01-02", it.NotBefore)
	if err != nil {
		return false
	}
	return now.Before(t)
}

var estimateTokenRe = regexp.MustCompile(`^\s*([0-9]*\.?[0-9]+)\s*(min|m|h|d|w)\b`)

func EstimateHours(s string) (float64, bool) {
	m := estimateTokenRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, false
	}
	switch m[2] {
	case "min", "m":
		return n / 60, true
	case "h":
		return n, true
	case "d":
		return n * 8, true
	case "w":
		return n * 40, true
	}
	return 0, false
}
