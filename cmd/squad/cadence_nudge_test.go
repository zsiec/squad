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

func TestCadenceNudgeText_ClaimMentionsThinking(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	got := cadenceNudgeText("claim", "")
	if !strings.Contains(got, "squad thinking") {
		t.Fatalf("claim text should mention `squad thinking`, got %q", got)
	}
	if !strings.Contains(got, "SQUAD_NO_CADENCE_NUDGES") {
		t.Fatalf("claim text should advertise the silence env var, got %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("text helper must not include a trailing newline, got %q", got)
	}
}

func TestCadenceNudgeText_DoneWithoutTypeIsEmpty(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	if got := cadenceNudgeText("done", ""); got != "" {
		t.Fatalf("done with no type should be empty, got %q", got)
	}
}

func TestCadenceNudgeText_DoneBugMentionsGotcha(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	got := cadenceNudgeText("done", "bug")
	if !strings.Contains(got, "gotcha") {
		t.Fatalf("done+bug should mention gotcha, got %q", got)
	}
	if !strings.Contains(got, "squad learning propose gotcha") {
		t.Fatalf("done+bug should mention `squad learning propose gotcha`, got %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("text helper must not include a trailing newline, got %q", got)
	}
}

func TestCadenceNudgeText_DoneFeatureOrTaskUsesGenericCopy(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	for _, typ := range []string{"feat", "feature", "task"} {
		t.Run("type="+typ, func(t *testing.T) {
			got := cadenceNudgeText("done", typ)
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

func TestCadenceNudgeText_DoneOverheadTypesAreEmpty(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	for _, typ := range []string{"chore", "tech-debt", "bet", ""} {
		t.Run("type="+typ, func(t *testing.T) {
			if got := cadenceNudgeText("done", typ); got != "" {
				t.Fatalf("done+%q should be empty, got %q", typ, got)
			}
		})
	}
}

func TestCadenceNudgeText_UnknownKindIsEmpty(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	if got := cadenceNudgeText("bogus", ""); got != "" {
		t.Fatalf("unknown kind should be empty, got %q", got)
	}
}

func TestCadenceNudgeText_SuppressedByEnv(t *testing.T) {
	for _, val := range []string{"1", "true", "TRUE"} {
		t.Run("env="+val, func(t *testing.T) {
			t.Setenv("SQUAD_NO_CADENCE_NUDGES", val)
			if got := cadenceNudgeText("claim", ""); got != "" {
				t.Fatalf("env=%q should suppress claim, got %q", val, got)
			}
			if got := cadenceNudgeText("done", "bug"); got != "" {
				t.Fatalf("env=%q should suppress done+bug, got %q", val, got)
			}
		})
	}
}

func TestCadenceNudgeText_MatchesPrintOutput(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	cases := []struct{ kind, typ string }{
		{"claim", ""},
		{"done", "bug"},
		{"done", "feat"},
		{"done", "feature"},
		{"done", "task"},
	}
	for _, c := range cases {
		t.Run(c.kind+"/"+c.typ, func(t *testing.T) {
			var buf bytes.Buffer
			printCadenceNudgeFor(&buf, c.kind, c.typ)
			got := cadenceNudgeText(c.kind, c.typ)
			want := strings.TrimRight(buf.String(), "\n")
			if got != want {
				t.Fatalf("text differs from print output:\n  text:  %q\n  print: %q", got, want)
			}
		})
	}
}

func TestSecondOpinionNudgeText_FiresForP0(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	got := secondOpinionNudgeText("P0", "low")
	if !strings.Contains(got, "squad ask @") {
		t.Fatalf("P0 should mention `squad ask @`, got %q", got)
	}
	if !strings.Contains(got, "SQUAD_NO_CADENCE_NUDGES") {
		t.Fatalf("text should advertise the silence env var, got %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("text helper must not include a trailing newline, got %q", got)
	}
}

func TestSecondOpinionNudgeText_FiresForP1(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	got := secondOpinionNudgeText("P1", "low")
	if !strings.Contains(got, "squad ask @") {
		t.Fatalf("P1 should mention `squad ask @`, got %q", got)
	}
}

func TestSecondOpinionNudgeText_FiresForHighRisk(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	got := secondOpinionNudgeText("P2", "high")
	if !strings.Contains(got, "squad ask @") {
		t.Fatalf("risk=high should mention `squad ask @`, got %q", got)
	}
}

func TestSecondOpinionNudgeText_QuietForP2Low(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	if got := secondOpinionNudgeText("P2", "low"); got != "" {
		t.Fatalf("P2+low should be empty, got %q", got)
	}
}

func TestSecondOpinionNudgeText_QuietForP3Low(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	if got := secondOpinionNudgeText("P3", "low"); got != "" {
		t.Fatalf("P3+low should be empty, got %q", got)
	}
}

func TestSecondOpinionNudgeText_RespectsSilence(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	if got := secondOpinionNudgeText("P0", "high"); got != "" {
		t.Fatalf("env=1 should suppress nudge even on P0+high, got %q", got)
	}
}

func TestSecondOpinionNudgeText_MatchesPrintOutput(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	cases := []struct{ priority, risk string }{
		{"P0", "low"},
		{"P1", "low"},
		{"P2", "high"},
		{"P2", "low"},
	}
	for _, c := range cases {
		t.Run(c.priority+"/"+c.risk, func(t *testing.T) {
			var buf bytes.Buffer
			printSecondOpinionNudge(&buf, c.priority, c.risk)
			got := secondOpinionNudgeText(c.priority, c.risk)
			want := strings.TrimRight(buf.String(), "\n")
			if got != want {
				t.Fatalf("text differs from print output:\n  text:  %q\n  print: %q", got, want)
			}
		})
	}
}

func TestMilestoneTargetNudgeText_FiresWhenAtLeastTwo(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	got := milestoneTargetNudgeText(4)
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
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("text helper must not include a trailing newline, got %q", got)
	}
}

func TestMilestoneTargetNudgeText_EmptyForLowCounts(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	for _, total := range []int{-1, 0, 1} {
		if got := milestoneTargetNudgeText(total); got != "" {
			t.Fatalf("acTotal=%d should be empty, got %q", total, got)
		}
	}
}

func TestMilestoneTargetNudgeText_RespectsSilence(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	if got := milestoneTargetNudgeText(4); got != "" {
		t.Fatalf("env=1 should suppress, got %q", got)
	}
}

func TestMilestoneTargetNudgeText_MatchesPrintOutput(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	for _, total := range []int{0, 1, 2, 5} {
		t.Run("", func(t *testing.T) {
			var buf bytes.Buffer
			printMilestoneTargetNudge(&buf, total)
			got := milestoneTargetNudgeText(total)
			want := strings.TrimRight(buf.String(), "\n")
			if got != want {
				t.Fatalf("text differs from print output (total=%d):\n  text:  %q\n  print: %q", total, got, want)
			}
		})
	}
}

func TestDecomposeNudgeText_FiresAboveBothThresholds(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	got := decomposeNudgeText("FEAT-100", 4, 3)
	if !strings.Contains(got, "4 AC items") {
		t.Errorf("output should mention AC count (4), got %q", got)
	}
	if !strings.Contains(got, "3 files") {
		t.Errorf("output should mention file-ref count (3), got %q", got)
	}
	if !strings.Contains(got, "squad decompose FEAT-100") {
		t.Errorf("output should suggest `squad decompose <id>`, got %q", got)
	}
	if !strings.Contains(got, "SQUAD_NO_CADENCE_NUDGES") {
		t.Errorf("output should advertise the silence env var, got %q", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("text helper must not include a trailing newline, got %q", got)
	}
}

func TestDecomposeNudgeText_QuietBelowThresholds(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	cases := []struct{ ac, refs int }{
		{3, 5}, {4, 2}, {0, 0}, {1, 1}, {3, 3},
	}
	for _, c := range cases {
		if got := decomposeNudgeText("FEAT-100", c.ac, c.refs); got != "" {
			t.Errorf("ac=%d refs=%d should be empty, got %q", c.ac, c.refs, got)
		}
	}
}

func TestDecomposeNudgeText_RespectsSilence(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	if got := decomposeNudgeText("FEAT-100", 10, 10); got != "" {
		t.Errorf("env=1 should suppress nudge even far above threshold, got %q", got)
	}
}

func TestDecomposeNudgeText_MatchesPrintOutput(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	var buf bytes.Buffer
	printDecomposeNudge(&buf, "FEAT-100", 4, 3)
	want := strings.TrimRight(buf.String(), "\n")
	if got := decomposeNudgeText("FEAT-100", 4, 3); got != want {
		t.Fatalf("text differs from print output:\n  text:  %q\n  print: %q", got, want)
	}
}
