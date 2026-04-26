package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func typeRunes(t *testing.T, m ReasonModal, s string) ReasonModal {
	t.Helper()
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestReasonModal_TypingBuildsValue(t *testing.T) {
	m := NewReasonModal()
	m = typeRunes(t, m, "hi")
	if got := m.Value(); got != "hi" {
		t.Fatalf("Value()=%q want %q", got, "hi")
	}
}

func TestReasonModal_EnterSubmitsValue(t *testing.T) {
	m := NewReasonModal()
	m = typeRunes(t, m, "duplicate")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Submitted() {
		t.Fatalf("Submitted()=false want true")
	}
	if got := m.Value(); got != "duplicate" {
		t.Fatalf("Value()=%q want %q", got, "duplicate")
	}
	if m.Cancelled() {
		t.Fatalf("Cancelled()=true want false")
	}
}

func TestReasonModal_EscCancels(t *testing.T) {
	m := NewReasonModal()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.Cancelled() {
		t.Fatalf("Cancelled()=false want true")
	}
	if m.Submitted() {
		t.Fatalf("Submitted()=true want false")
	}
}

func TestReasonModal_EnterOnEmptyShowsError(t *testing.T) {
	m := NewReasonModal()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Submitted() {
		t.Fatalf("Submitted()=true want false on empty submit")
	}
	if got := m.Error(); !strings.Contains(got, "required") {
		t.Fatalf("Error()=%q want contains 'required'", got)
	}
}

func TestReasonModal_BackspaceTrimsValue(t *testing.T) {
	m := NewReasonModal()
	m = typeRunes(t, m, "abc")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if got := m.Value(); got != "ab" {
		t.Fatalf("Value()=%q want %q", got, "ab")
	}
}

func TestReasonModal_View(t *testing.T) {
	m := NewReasonModal()
	m = typeRunes(t, m, "dup")
	out := m.View()
	if !strings.Contains(out, "Reject reason") {
		t.Fatalf("View missing 'Reject reason': %q", out)
	}
	if !strings.Contains(out, "dup") {
		t.Fatalf("View missing current value 'dup': %q", out)
	}
}

func TestReasonModal_TypingClearsError(t *testing.T) {
	m := NewReasonModal()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Error() == "" {
		t.Fatalf("expected error after empty submit")
	}
	m = typeRunes(t, m, "x")
	if m.Error() != "" {
		t.Fatalf("Error()=%q want empty after typing", m.Error())
	}
}

func TestReasonModal_NoUpdatesAfterSubmit(t *testing.T) {
	m := NewReasonModal()
	m = typeRunes(t, m, "ok")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.Submitted() {
		t.Fatalf("not submitted")
	}
	m2 := typeRunes(t, m, "more")
	if m2.Value() != "ok" {
		t.Fatalf("value mutated post-submit: %q", m2.Value())
	}
}

func TestReasonModal_SpaceAddsSpace(t *testing.T) {
	m := NewReasonModal()
	m = typeRunes(t, m, "a")
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = typeRunes(t, m, "b")
	if got := m.Value(); got != "a b" {
		t.Fatalf("Value()=%q want %q", got, "a b")
	}
}
