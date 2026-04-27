package intake

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"testing"
)

func openTestSession(t *testing.T) (*sql.DB, string, string) {
	t.Helper()
	d := newTestDB(t)
	s, _, _, err := Open(context.Background(), d, OpenParams{
		RepoID:   "repo-a",
		AgentID:  "agent-1",
		Mode:     ModeNew,
		IdeaSeed: "make logging searchable",
	})
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	return d, s.ID, s.AgentID
}

func TestAppendTurn_MonotonicSeq(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	seq1, _, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "first", nil)
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if seq1 != 1 {
		t.Errorf("seq1 = %d; want 1", seq1)
	}

	seq2, _, err := AppendTurn(ctx, d, checklist, sid, aid, "agent", "second", nil)
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if seq2 != 2 {
		t.Errorf("seq2 = %d; want 2", seq2)
	}

	seq3, _, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "third", nil)
	if err != nil {
		t.Fatalf("turn 3: %v", err)
	}
	if seq3 != 3 {
		t.Errorf("seq3 = %d; want 3", seq3)
	}
}

func TestAppendTurn_StillRequiredShrinks_ItemOnly(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	_, prev, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "rough idea", nil)
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	sort.Strings(prev)
	if want := []string{"acceptance", "area", "intent", "title"}; !equalSlices(prev, want) {
		t.Errorf("after no fills: still=%v want %v", prev, want)
	}

	_, after2, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "title is X", []string{"title"})
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if contains2(after2, "title") {
		t.Errorf("title should be drained: %v", after2)
	}
	if len(after2) > len(prev) {
		t.Errorf("still grew across turns 1→2: %d → %d", len(prev), len(after2))
	}
	prev = after2

	_, after3, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "rest", []string{"intent", "acceptance", "area"})
	if err != nil {
		t.Fatalf("turn 3: %v", err)
	}
	if len(after3) != 0 {
		t.Errorf("everything filled: still=%v want empty", after3)
	}
	if len(after3) > len(prev) {
		t.Errorf("still grew across turns 2→3: %d → %d", len(prev), len(after3))
	}
}

func TestAppendTurn_ShapeInferredFromDottedField(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	_, still, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "shaping", []string{"spec.title"})
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if !contains2(still, "spec.motivation") {
		t.Errorf("expected spec_epic_items shape with spec.motivation in still: %v", still)
	}
	if contains2(still, "spec.title") {
		t.Errorf("filled spec.title leaked into still: %v", still)
	}
	if contains2(still, "title") || contains2(still, "intent") {
		t.Errorf("flat item_only fields leaked into spec_epic_items still: %v", still)
	}
}

func TestAppendTurn_RejectsEmptyContent(t *testing.T) {
	d, sid, aid := openTestSession(t)
	checklist, _ := LoadChecklist("")
	_, _, err := AppendTurn(context.Background(), d, checklist, sid, aid, "user", "   ", nil)
	if err == nil {
		t.Fatalf("expected error on whitespace-only content")
	}
}

func TestAppendTurn_RejectsInvalidRole(t *testing.T) {
	d, sid, aid := openTestSession(t)
	checklist, _ := LoadChecklist("")
	_, _, err := AppendTurn(context.Background(), d, checklist, sid, aid, "robot", "hi", nil)
	if err == nil {
		t.Fatalf("expected error on invalid role")
	}
}

func TestAppendTurn_RejectsClosedSession(t *testing.T) {
	d, sid, aid := openTestSession(t)
	if err := Cancel(context.Background(), d, sid, aid); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	checklist, _ := LoadChecklist("")
	_, _, err := AppendTurn(context.Background(), d, checklist, sid, aid, "user", "after cancel", nil)
	if !errors.Is(err, ErrIntakeAlreadyClosed) {
		t.Fatalf("got %v; want ErrIntakeAlreadyClosed", err)
	}
}

func TestAppendTurn_RejectsForeignAgent(t *testing.T) {
	d, sid, _ := openTestSession(t)
	checklist, _ := LoadChecklist("")
	_, _, err := AppendTurn(context.Background(), d, checklist, sid, "other-agent", "user", "hi", nil)
	if !errors.Is(err, ErrIntakeNotYours) {
		t.Fatalf("got %v; want ErrIntakeNotYours", err)
	}
}

func TestAppendTurn_RejectsUnknownSession(t *testing.T) {
	d := newTestDB(t)
	checklist, _ := LoadChecklist("")
	_, _, err := AppendTurn(context.Background(), d, checklist, "intake-19990101-deadbeefcafe", "agent-1", "user", "hi", nil)
	if !errors.Is(err, ErrIntakeNotFound) {
		t.Fatalf("got %v; want ErrIntakeNotFound", err)
	}
}

func TestAppendTurn_ShapeLocksAndRejectsConflict(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	_, stillTurn1, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "flat title", []string{"title"})
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}

	_, _, err = AppendTurn(ctx, d, checklist, sid, aid, "user", "actually a spec", []string{"spec.title"})
	if err == nil {
		t.Fatalf("expected shape-conflict rejection on dotted-after-flat")
	}

	_, stillTurn3, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "more flat", []string{"intent"})
	if err != nil {
		t.Fatalf("turn 3: %v", err)
	}
	if len(stillTurn3) > len(stillTurn1) {
		t.Fatalf("still grew across compatible turns: %d -> %d", len(stillTurn1), len(stillTurn3))
	}
}

func TestAppendTurn_DottedFirstLocksSpecEpicItems(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	_, stillTurn1, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "spec start", []string{"spec.title"})
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if contains2(stillTurn1, "title") {
		t.Errorf("flat 'title' leaked into spec_epic_items still: %v", stillTurn1)
	}

	if _, _, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "later flat", []string{"intent"}); err == nil {
		t.Fatalf("expected shape-conflict rejection on flat-after-dotted")
	}

	_, stillTurn3, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "more spec", []string{"spec.motivation"})
	if err != nil {
		t.Fatalf("turn 3: %v", err)
	}
	if len(stillTurn3) > len(stillTurn1) {
		t.Fatalf("still grew across compatible spec_epic_items turns: %d -> %d", len(stillTurn1), len(stillTurn3))
	}
}

func TestAppendTurn_RejectsMixedFieldsInSingleTurn(t *testing.T) {
	d, sid, aid := openTestSession(t)
	checklist, _ := LoadChecklist("")
	_, _, err := AppendTurn(context.Background(), d, checklist, sid, aid, "user", "mixed", []string{"title", "spec.motivation"})
	if err == nil {
		t.Fatalf("expected error on mixed flat+dotted fields in one turn")
	}
}

func TestAppendTurn_RejectsCancelledSessionInsideTx(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	if err := Cancel(ctx, d, sid, aid); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	_, _, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "after cancel", nil)
	if !errors.Is(err, ErrIntakeAlreadyClosed) {
		t.Fatalf("got %v; want ErrIntakeAlreadyClosed", err)
	}
	var n int
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM intake_turns WHERE session_id=?`, sid,
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected zero turns on cancelled session; got %d", n)
	}
}

// equalSlices compares two []string for set-equal (after sorting).
func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

func contains2(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
