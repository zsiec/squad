package items

import "testing"

func TestWriteFeedback_InsertsAboveProblem(t *testing.T) {
	in := "## Problem\nfoo\n\n## Acceptance criteria\n- [ ] x\n"
	got := WriteFeedback(in, "rename area")
	want := "## Reviewer feedback\nrename area\n\n## Problem\nfoo\n\n## Acceptance criteria\n- [ ] x\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestWriteFeedback_ReplacesExisting(t *testing.T) {
	in := "## Reviewer feedback\nold note\n\n## Problem\nfoo\n"
	got := WriteFeedback(in, "new note")
	want := "## Reviewer feedback\nnew note\n\n## Problem\nfoo\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestWriteFeedback_NoProblemSection_PrependsAtTop(t *testing.T) {
	in := "freeform body without sections\n"
	got := WriteFeedback(in, "needs structure")
	want := "## Reviewer feedback\nneeds structure\n\nfreeform body without sections\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestWriteFeedback_TrailingNewlinePreserved(t *testing.T) {
	in := "## Problem\nfoo\n"
	got := WriteFeedback(in, "fix")
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Fatalf("expected trailing newline, got %q", got)
	}
}

func TestMoveFeedbackToHistory_FirstRound(t *testing.T) {
	in := "## Reviewer feedback\nplease tighten\n\n## Problem\nfoo\n"
	got := MoveFeedbackToHistory(in, "2026-04-26")
	want := "## Refinement history\n### Round 1 — 2026-04-26\nplease tighten\n\n## Problem\nfoo\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestMoveFeedbackToHistory_AppendsRound(t *testing.T) {
	in := "## Reviewer feedback\nround 2 ask\n\n## Refinement history\n### Round 1 — 2026-04-25\nfirst ask\n\n## Problem\nfoo\n"
	got := MoveFeedbackToHistory(in, "2026-04-26")
	want := "## Refinement history\n### Round 1 — 2026-04-25\nfirst ask\n\n### Round 2 — 2026-04-26\nround 2 ask\n\n## Problem\nfoo\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestMoveFeedbackToHistory_NoFeedback_Noop(t *testing.T) {
	in := "## Problem\nfoo\n"
	got := MoveFeedbackToHistory(in, "2026-04-26")
	if got != in {
		t.Fatalf("expected no-op, got: %q", got)
	}
}
