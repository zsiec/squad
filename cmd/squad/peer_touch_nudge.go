package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/touch"
)

const peerTouchNudgeMaxLines = 3

// peerTouchOverlapNudgeText returns the post-claim multi-line warning when
// any of the agent's claimed-item file refs overlaps a peer's still-open
// touch within the freshness window. Empty when silenced or no overlaps.
//
// Caller is expected to have already filtered touches by freshness via
// touch.Tracker.ListOthersSince — this function only intersects.
//
// More than peerTouchNudgeMaxLines overlaps collapse to "and N more" with
// a pointer to `squad touches list-others`, matching the AC.
func peerTouchOverlapNudgeText(itemFileRefs []string, peerTouches []touch.ActiveTouch, now time.Time) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if len(itemFileRefs) == 0 || len(peerTouches) == 0 {
		return ""
	}
	want := make(map[string]struct{}, len(itemFileRefs))
	for _, p := range itemFileRefs {
		want[p] = struct{}{}
	}
	var overlaps []touch.ActiveTouch
	for _, t := range peerTouches {
		if _, ok := want[t.Path]; ok {
			overlaps = append(overlaps, t)
		}
	}
	if len(overlaps) == 0 {
		return ""
	}
	var b strings.Builder
	for i, ov := range overlaps {
		if i >= peerTouchNudgeMaxLines {
			break
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "  squad: heads up — %s is touching %s (last %s)",
			ov.AgentID, ov.Path, peerTouchAge(now, ov.StartedAt))
	}
	if extra := len(overlaps) - peerTouchNudgeMaxLines; extra > 0 {
		fmt.Fprintf(&b, "\n  squad: and %d more — squad touches list-others to see all", extra)
	}
	return b.String()
}

func printPeerTouchOverlapNudge(w io.Writer, itemFileRefs []string, peerTouches []touch.ActiveTouch, now time.Time) {
	if t := peerTouchOverlapNudgeText(itemFileRefs, peerTouches, now); t != "" {
		fmt.Fprintln(w, t)
	}
}

// peerTouchAge formats now-startedAt for the (last X) suffix. Always
// produces a positive integer; sub-minute deltas round up to "1m" so the
// nudge never reads "0m." Bounded to "Xh" once an hour passes.
func peerTouchAge(now time.Time, startedAt int64) string {
	d := now.Sub(time.Unix(startedAt, 0))
	if d < time.Minute {
		return "1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}
