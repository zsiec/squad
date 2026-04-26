package main

import (
	"fmt"
	"io"
	"os"
)

// printCadenceNudge writes a one-line reminder to w pointing the agent at
// the right chat verb for the moment. Suppressed when the env var is set
// truthy — scripts and CI runs don't want the noise.
//
// AGENTS.md tells agents to post on claim, on commit, on done, etc. The
// nudge is the in-flow reminder so the rule reaches the agent without
// requiring them to re-read the manual mid-loop.
func printCadenceNudge(w io.Writer, kind string) {
	if cadenceNudgesSilenced() {
		return
	}
	switch kind {
	case "claim":
		fmt.Fprintln(w, "  tip: `squad thinking <msg>` to share intent · silence with SQUAD_NO_CADENCE_NUDGES=1")
	case "done":
		fmt.Fprintln(w, "  tip: surprised by anything? `squad learning propose <kind> <slug>` · silence with SQUAD_NO_CADENCE_NUDGES=1")
	}
}

func cadenceNudgesSilenced() bool {
	v := os.Getenv("SQUAD_NO_CADENCE_NUDGES")
	return v == "1" || v == "true" || v == "TRUE"
}
