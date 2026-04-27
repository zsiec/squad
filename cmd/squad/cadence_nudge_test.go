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

func TestPrintCadenceNudge_DoneWithoutTypeIsSilent(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	// The 2-arg wrapper passes itemType="" — overhead/unknown types are
	// silent under the type-aware contract. Done call sites that want a
	// nudge must call printCadenceNudgeFor with the actual item type.
	printCadenceNudge(&buf, "done")
	if buf.Len() != 0 {
		t.Fatalf("done with no type should be silent, got %q", buf.String())
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

func TestPrintCadenceNudgeFor_DoneBugMentionsGotcha(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printCadenceNudgeFor(&buf, "done", "bug")
	got := buf.String()
	if !strings.Contains(got, "gotcha") {
		t.Fatalf("done+bug should mention gotcha, got %q", got)
	}
	if !strings.Contains(got, "squad learning propose gotcha") {
		t.Fatalf("done+bug should mention `squad learning propose gotcha`, got %q", got)
	}
}

func TestPrintCadenceNudgeFor_DoneFeatureOrTaskUsesGenericCopy(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	for _, typ := range []string{"feat", "feature", "task"} {
		t.Run("type="+typ, func(t *testing.T) {
			var buf bytes.Buffer
			printCadenceNudgeFor(&buf, "done", typ)
			got := buf.String()
			if !strings.Contains(got, "surprised by anything?") {
				t.Fatalf("done+%s should use generic copy, got %q", typ, got)
			}
			if !strings.Contains(got, "squad learning propose") {
				t.Fatalf("done+%s should mention `squad learning propose`, got %q", typ, got)
			}
			if strings.Contains(got, "gotcha") {
				t.Fatalf("done+%s should NOT mention gotcha, got %q", typ, got)
			}
		})
	}
}

func TestPrintCadenceNudgeFor_DoneOverheadTypesAreSilent(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	for _, typ := range []string{"chore", "tech-debt", "bet", ""} {
		t.Run("type="+typ, func(t *testing.T) {
			var buf bytes.Buffer
			printCadenceNudgeFor(&buf, "done", typ)
			if buf.Len() != 0 {
				t.Fatalf("done+%q should print nothing, got %q", typ, buf.String())
			}
		})
	}
}

func TestPrintCadenceNudgeFor_SuppressedByEnv(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	var buf bytes.Buffer
	printCadenceNudgeFor(&buf, "done", "bug")
	printCadenceNudgeFor(&buf, "done", "feat")
	printCadenceNudgeFor(&buf, "claim", "")
	if buf.Len() != 0 {
		t.Fatalf("env=1 should suppress all variants, got %q", buf.String())
	}
}

func TestPrintSecondOpinionNudge_FiresForP0(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printSecondOpinionNudge(&buf, "P0", "low")
	got := buf.String()
	if !strings.Contains(got, "squad ask @") {
		t.Fatalf("P0 should mention `squad ask @`, got %q", got)
	}
	if !strings.Contains(got, "SQUAD_NO_CADENCE_NUDGES") {
		t.Fatalf("nudge should advertise the silence env var, got %q", got)
	}
}

func TestPrintSecondOpinionNudge_FiresForP1(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printSecondOpinionNudge(&buf, "P1", "low")
	got := buf.String()
	if !strings.Contains(got, "squad ask @") {
		t.Fatalf("P1 should mention `squad ask @`, got %q", got)
	}
}

func TestPrintSecondOpinionNudge_FiresForHighRisk(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printSecondOpinionNudge(&buf, "P2", "high")
	got := buf.String()
	if !strings.Contains(got, "squad ask @") {
		t.Fatalf("risk=high should mention `squad ask @`, got %q", got)
	}
}

func TestPrintSecondOpinionNudge_QuietForP2Low(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printSecondOpinionNudge(&buf, "P2", "low")
	if buf.Len() != 0 {
		t.Fatalf("P2+low should be silent, got %q", buf.String())
	}
}

func TestPrintSecondOpinionNudge_QuietForP3Low(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printSecondOpinionNudge(&buf, "P3", "low")
	if buf.Len() != 0 {
		t.Fatalf("P3+low should be silent, got %q", buf.String())
	}
}

func TestPrintSecondOpinionNudge_RespectsSilence(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	var buf bytes.Buffer
	printSecondOpinionNudge(&buf, "P0", "high")
	if buf.Len() != 0 {
		t.Fatalf("env=1 should suppress nudge even on P0+high, got %q", buf.String())
	}
}

func TestPrintMilestoneTargetNudge_FiresWhenAtLeastTwo(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printMilestoneTargetNudge(&buf, 4)
	got := buf.String()
	if !strings.Contains(got, "4 AC") {
		t.Fatalf("output should contain the AC total (4), got %q", got)
	}
	if !strings.Contains(got, "~4") {
		t.Fatalf("output should mention ~4 milestone posts, got %q", got)
	}
	if !strings.Contains(got, "squad milestone") {
		t.Fatalf("output should mention `squad milestone`, got %q", got)
	}
	if !strings.Contains(got, "SQUAD_NO_CADENCE_NUDGES") {
		t.Fatalf("output should advertise the silence env var, got %q", got)
	}
}

func TestPrintMilestoneTargetNudge_SilentForLowCounts(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	for _, total := range []int{0, 1} {
		var buf bytes.Buffer
		printMilestoneTargetNudge(&buf, total)
		if buf.Len() != 0 {
			t.Fatalf("acTotal=%d should be silent, got %q", total, buf.String())
		}
	}
}

func TestPrintMilestoneTargetNudge_NegativeCountSilent(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printMilestoneTargetNudge(&buf, -1)
	if buf.Len() != 0 {
		t.Fatalf("negative acTotal should be silent, got %q", buf.String())
	}
}

func TestPrintMilestoneTargetNudge_RespectsSilence(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	var buf bytes.Buffer
	printMilestoneTargetNudge(&buf, 4)
	if buf.Len() != 0 {
		t.Fatalf("env=1 should suppress, got %q", buf.String())
	}
}
