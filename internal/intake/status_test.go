package intake

import (
	"context"
	"errors"
	"testing"
)

func TestStatus_ReturnsFullTranscript(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	if _, _, err := AppendTurn(ctx, d, checklist, sid, aid, "agent", "what's the title?", nil); err != nil {
		t.Fatalf("turn 1: %v", err)
	}
	if _, _, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "make logging searchable", []string{"title"}); err != nil {
		t.Fatalf("turn 2: %v", err)
	}
	if _, _, err := AppendTurn(ctx, d, checklist, sid, aid, "agent", "intent?", nil); err != nil {
		t.Fatalf("turn 3: %v", err)
	}

	out, err := Status(ctx, d, checklist, sid, aid)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if out.Session.ID != sid {
		t.Errorf("session.ID = %q; want %q", out.Session.ID, sid)
	}
	if got, want := len(out.Transcript), 3; got != want {
		t.Fatalf("transcript len = %d; want %d", got, want)
	}
	for i, turn := range out.Transcript {
		if turn.Seq != i+1 {
			t.Errorf("transcript[%d].Seq = %d; want %d", i, turn.Seq, i+1)
		}
	}
	if out.Transcript[1].Role != "user" || out.Transcript[1].Content != "make logging searchable" {
		t.Errorf("turn 2 mismatch: %+v", out.Transcript[1])
	}
}

func TestStatus_StillRequiredReflectsFills(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	if _, _, err := AppendTurn(ctx, d, checklist, sid, aid, "user", "title fill", []string{"title"}); err != nil {
		t.Fatalf("turn: %v", err)
	}
	out, err := Status(ctx, d, checklist, sid, aid)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, f := range out.StillRequired {
		if f == "title" {
			t.Errorf("title should be drained from still_required; got %v", out.StillRequired)
		}
	}
	want := []string{"intent", "acceptance", "area"}
	if len(out.StillRequired) != len(want) {
		t.Errorf("still len = %d; want %d (%v)", len(out.StillRequired), len(want), out.StillRequired)
	}
}

func TestStatus_RejectsForeignAgent(t *testing.T) {
	d, sid, _ := openTestSession(t)
	checklist, _ := LoadChecklist("")
	_, err := Status(context.Background(), d, checklist, sid, "other-agent")
	if !errors.Is(err, ErrIntakeNotYours) {
		t.Fatalf("got %v; want ErrIntakeNotYours", err)
	}
}

func TestStatus_RejectsUnknownSession(t *testing.T) {
	d := newTestDB(t)
	checklist, _ := LoadChecklist("")
	_, err := Status(context.Background(), d, checklist, "intake-19990101-deadbeefcafe", "agent-1")
	if !errors.Is(err, ErrIntakeNotFound) {
		t.Fatalf("got %v; want ErrIntakeNotFound", err)
	}
}

func TestStatus_ReturnsForCancelledSession(t *testing.T) {
	d, sid, aid := openTestSession(t)
	ctx := context.Background()
	checklist, _ := LoadChecklist("")

	if err := Cancel(ctx, d, sid, aid); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	out, err := Status(ctx, d, checklist, sid, aid)
	if err != nil {
		t.Fatalf("status on cancelled session should still return: %v", err)
	}
	if out.Session.Status != StatusCancelled {
		t.Errorf("session.Status = %q; want %q", out.Session.Status, StatusCancelled)
	}
}
