package main

import (
	"fmt"
	"io"
	"os"
)

// cadenceNudgeText returns the one-line claim/done reminder pointing the
// agent at the right chat verb for the moment, or "" when silenced or when
// no copy applies. The string never carries a trailing newline — the print
// wrappers add it; the MCP handlers carry the bare line into a JSON array.
//
// AGENTS.md tells agents to post on claim, on commit, on done, etc. The
// nudge is the in-flow reminder so the rule reaches the agent without
// requiring them to re-read the manual mid-loop.
func cadenceNudgeText(kind, itemType string) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	switch kind {
	case "claim":
		return "  tip: `squad thinking <msg>` to share intent · silence with SQUAD_NO_CADENCE_NUDGES=1"
	case "done":
		switch itemType {
		case "bug":
			return "  tip: gotcha worth filing? `squad learning propose gotcha <slug>` · silence with SQUAD_NO_CADENCE_NUDGES=1"
		case "feat", "feature", "task":
			return "  tip: surprised by anything? `squad learning propose <kind> <slug>` · silence with SQUAD_NO_CADENCE_NUDGES=1"
		}
	}
	return ""
}

// secondOpinionNudgeText returns the high-stakes-claim peer-check nudge,
// or "" when silenced or when neither priority nor risk warrant it.
// Redirecting at claim-time is much cheaper than at review.
func secondOpinionNudgeText(priority, risk string) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if priority != "P0" && priority != "P1" && risk != "high" {
		return ""
	}
	return "  tip: high-stakes claim — consider `squad ask @<peer> \"sanity-check my approach?\"` before starting · silence with SQUAD_NO_CADENCE_NUDGES=1"
}

// milestoneTargetNudgeText returns the AC-target nudge naming the AC total
// at claim-time so the agent has a concrete number to compare against while
// working — chat-cadence says "milestone each AC" but the dogfood data
// showed agents posting at most one milestone per item. Empty for 0 or 1
// AC items where a per-AC target adds no signal.
func milestoneTargetNudgeText(acTotal int) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if acTotal < 2 {
		return ""
	}
	return fmt.Sprintf("  tip: %d AC items — expect ~%d 'squad milestone' posts as you green each one · silence with SQUAD_NO_CADENCE_NUDGES=1", acTotal, acTotal)
}

func printCadenceNudge(w io.Writer, kind string) {
	printCadenceNudgeFor(w, kind, "")
}

func printCadenceNudgeFor(w io.Writer, kind, itemType string) {
	if t := cadenceNudgeText(kind, itemType); t != "" {
		fmt.Fprintln(w, t)
	}
}

func cadenceNudgesSilenced() bool {
	v := os.Getenv("SQUAD_NO_CADENCE_NUDGES")
	return v == "1" || v == "true" || v == "TRUE"
}

func printSecondOpinionNudge(w io.Writer, priority, risk string) {
	if t := secondOpinionNudgeText(priority, risk); t != "" {
		fmt.Fprintln(w, t)
	}
}

func printMilestoneTargetNudge(w io.Writer, acTotal int) {
	if t := milestoneTargetNudgeText(acTotal); t != "" {
		fmt.Fprintln(w, t)
	}
}

// worktreeNudgeText returns the post-claim cd hint when the claim provisioned
// an isolated worktree. Empty when silenced or path is empty so the caller
// can branch on a single value.
func worktreeNudgeText(path string) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if path == "" {
		return ""
	}
	return "  tip: cd into the isolated worktree: cd " + path
}

func printWorktreeNudge(w io.Writer, path string) {
	if t := worktreeNudgeText(path); t != "" {
		fmt.Fprintln(w, t)
	}
}

// quickFollowupNudgeText returns the one-line reminder that the auto-derived
// stub from `squad learning quick` still has placeholder sections worth
// filling in when the agent has a moment. Empty when silenced. The print
// wrapper adds the newline; MCP carries the bare line into Tips.
func quickFollowupNudgeText() string {
	if cadenceNudgesSilenced() {
		return ""
	}
	return "  tip: edit the stub when you can — sections are placeholders · silence with SQUAD_NO_CADENCE_NUDGES=1"
}

func printQuickFollowupNudge(w io.Writer) {
	if t := quickFollowupNudgeText(); t != "" {
		fmt.Fprintln(w, t)
	}
}
