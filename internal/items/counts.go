package items

import "time"

type CountReport struct {
	InProgress int
	Ready      int
	Blocked    int
	Done       int
}

func Counts(w WalkResult, now time.Time) CountReport {
	statusByID := map[string]string{}
	for _, it := range w.Active {
		statusByID[it.ID] = it.Status
	}
	for _, it := range w.Done {
		statusByID[it.ID] = "done"
	}
	var c CountReport
	for _, it := range w.Active {
		switch {
		case it.Status == "in_progress":
			c.InProgress++
		case it.Status == "blocked":
			c.Blocked++
		case gatedUntil(it, now) || hasOpenBlocker(it, statusByID):
			c.Blocked++
		default:
			c.Ready++
		}
	}
	c.Done = len(w.Done)
	return c
}
