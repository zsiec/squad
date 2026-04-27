package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/touch"
)

func TestPeerTouchOverlapNudgeText_FiresOnOverlap(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	got := peerTouchOverlapNudgeText(
		[]string{"cmd/squad/claim.go", "internal/items/dor.go"},
		[]touch.ActiveTouch{
			{AgentID: "agent-bbf6", Path: "cmd/squad/claim.go", StartedAt: now.Add(-4 * time.Hour).Unix()},
		},
		now,
	)
	if !strings.Contains(got, "agent-bbf6") {
		t.Errorf("must name peer agent, got %q", got)
	}
	if !strings.Contains(got, "cmd/squad/claim.go") {
		t.Errorf("must name overlapping path, got %q", got)
	}
	if !strings.Contains(got, "(last 4h)") {
		t.Errorf("must include freshness suffix (last 4h), got %q", got)
	}
	if !strings.Contains(got, "heads up") {
		t.Errorf("must use 'heads up' lead, got %q", got)
	}
}

func TestPeerTouchOverlapNudgeText_QuietOnNoOverlap(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	got := peerTouchOverlapNudgeText(
		[]string{"cmd/squad/claim.go"},
		[]touch.ActiveTouch{
			{AgentID: "agent-bbf6", Path: "internal/server/inbox.go", StartedAt: now.Unix()},
		},
		now,
	)
	if got != "" {
		t.Errorf("non-overlapping touches should be silent, got %q", got)
	}
}

func TestPeerTouchOverlapNudgeText_QuietOnEmptyInputs(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	if got := peerTouchOverlapNudgeText(nil, nil, now); got != "" {
		t.Errorf("nil inputs should be silent, got %q", got)
	}
	if got := peerTouchOverlapNudgeText([]string{"a.go"}, nil, now); got != "" {
		t.Errorf("nil touches should be silent, got %q", got)
	}
	if got := peerTouchOverlapNudgeText(nil, []touch.ActiveTouch{{Path: "a.go"}}, now); got != "" {
		t.Errorf("nil refs should be silent, got %q", got)
	}
}

func TestPeerTouchOverlapNudgeText_CapsAtThreeAndCollapses(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	refs := []string{"a.go", "b.go", "c.go", "d.go", "e.go"}
	touches := []touch.ActiveTouch{
		{AgentID: "agent-x", Path: "a.go", StartedAt: now.Add(-1 * time.Hour).Unix()},
		{AgentID: "agent-x", Path: "b.go", StartedAt: now.Add(-1 * time.Hour).Unix()},
		{AgentID: "agent-x", Path: "c.go", StartedAt: now.Add(-1 * time.Hour).Unix()},
		{AgentID: "agent-x", Path: "d.go", StartedAt: now.Add(-1 * time.Hour).Unix()},
		{AgentID: "agent-x", Path: "e.go", StartedAt: now.Add(-1 * time.Hour).Unix()},
	}
	got := peerTouchOverlapNudgeText(refs, touches, now)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != peerTouchNudgeMaxLines+1 {
		t.Fatalf("expected %d capped lines + 1 collapse, got %d:\n%s", peerTouchNudgeMaxLines, len(lines), got)
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, "and 2 more") {
		t.Errorf("collapse line must say 'and 2 more', got %q", last)
	}
	if !strings.Contains(last, "squad touches list-others") {
		t.Errorf("collapse line must point at `squad touches list-others`, got %q", last)
	}
}

func TestPeerTouchOverlapNudgeText_RespectsSilenceEnv(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	got := peerTouchOverlapNudgeText(
		[]string{"a.go"},
		[]touch.ActiveTouch{{AgentID: "agent-x", Path: "a.go", StartedAt: now.Unix()}},
		now,
	)
	if got != "" {
		t.Errorf("env=1 should suppress, got %q", got)
	}
}

func TestPeerTouchAge_Formats(t *testing.T) {
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{30 * time.Second, "1m"},
		{5 * time.Minute, "5m"},
		{59 * time.Minute, "59m"},
		{1 * time.Hour, "1h"},
		{4 * time.Hour, "4h"},
		{23*time.Hour + 30*time.Minute, "23h"},
	}
	for _, c := range cases {
		got := peerTouchAge(now, now.Add(-c.ago).Unix())
		if got != c.want {
			t.Errorf("peerTouchAge(%s ago) = %q, want %q", c.ago, got, c.want)
		}
	}
}

func TestPrintPeerTouchOverlapNudge_MatchesText(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "")
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	refs := []string{"x.go"}
	touches := []touch.ActiveTouch{{AgentID: "agent-x", Path: "x.go", StartedAt: now.Add(-2 * time.Hour).Unix()}}
	var buf bytes.Buffer
	printPeerTouchOverlapNudge(&buf, refs, touches, now)
	want := peerTouchOverlapNudgeText(refs, touches, now) + "\n"
	if buf.String() != want {
		t.Errorf("print=%q text=%q (mismatch)", buf.String(), want)
	}
}
