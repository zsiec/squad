package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintCadenceNudge_ClaimEmitsThinkingTip(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printCadenceNudge(&buf, "claim")
	got := buf.String()
	if !strings.Contains(got, "squad thinking") {
		t.Fatalf("claim nudge should mention `squad thinking`, got %q", got)
	}
	if !strings.Contains(got, "SQUAD_NO_CADENCE_NUDGES") {
		t.Fatalf("claim nudge should advertise the silence env var, got %q", got)
	}
}

func TestPrintCadenceNudge_DoneEmitsLearningTip(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printCadenceNudge(&buf, "done")
	got := buf.String()
	if !strings.Contains(got, "squad learning propose") {
		t.Fatalf("done nudge should mention `squad learning propose`, got %q", got)
	}
}

func TestPrintCadenceNudge_SuppressedByEnv(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE"} {
		t.Run("env="+val, func(t *testing.T) {
			t.Setenv("SQUAD_NO_CADENCE_NUDGES", val)
			var buf bytes.Buffer
			printCadenceNudge(&buf, "claim")
			printCadenceNudge(&buf, "done")
			if buf.Len() != 0 {
				t.Fatalf("nudge should be suppressed when env=%q, got %q", val, buf.String())
			}
		})
	}
}

func TestPrintCadenceNudge_UnknownKindIsNoOp(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printCadenceNudge(&buf, "bogus")
	if buf.Len() != 0 {
		t.Fatalf("unknown kind should print nothing, got %q", buf.String())
	}
}
