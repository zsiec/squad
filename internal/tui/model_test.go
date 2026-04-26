package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
)

func TestModel_NumberKeysSwitchView(t *testing.T) {
	m := NewModel(nil, nil, "test-repo")
	if m.Current() != ViewItems {
		t.Fatalf("default view=%v want ViewItems", m.Current())
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	mm := updated.(Model)
	if mm.Current() != ViewAgents {
		t.Fatalf("after '2' view=%v want ViewAgents", mm.Current())
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}})
	mm = updated.(Model)
	if mm.Current() != ViewEpics {
		t.Fatalf("after '9' view=%v want ViewEpics", mm.Current())
	}
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	mm = updated.(Model)
	if mm.Current() != ViewStats {
		t.Fatalf("after '0' view=%v want ViewStats", mm.Current())
	}
}

func TestModel_ColonOpensPalette(t *testing.T) {
	m := NewModel(nil, nil, "test-repo")
	if m.PaletteActive() {
		t.Fatal("palette should be inactive on construction")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	mm := updated.(Model)
	if !mm.PaletteActive() {
		t.Fatal("palette should be active after ':'")
	}
}

func TestModel_EscClosesPalette(t *testing.T) {
	m := NewModel(nil, nil, "test-repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	mm := updated.(Model)
	updated, _ = mm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	mm = updated.(Model)
	if mm.PaletteActive() {
		t.Fatal("palette should close on Esc")
	}
}

func TestModel_PaletteSelectionSwitchesView(t *testing.T) {
	m := NewModel(nil, nil, "test-repo")
	updated, _ := m.Update(paletteSelMsgFor("agents"))
	mm := updated.(Model)
	if mm.Current() != ViewAgents {
		t.Fatalf("after palette selecting 'agents', current=%v", mm.Current())
	}
}

func TestModel_WindowSizeMsgUpdatesDims(t *testing.T) {
	m := NewModel(nil, nil, "test-repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	mm := updated.(Model)
	if mm.Width() != 120 || mm.Height() != 40 {
		t.Fatalf("size=%dx%d want 120x40", mm.Width(), mm.Height())
	}
}

func TestModel_ViewIncludesStatusBar(t *testing.T) {
	m := NewModel(nil, nil, "test-repo")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	mm := updated.(Model)
	out := mm.View()
	if !strings.Contains(out, "scope:test-repo") {
		t.Fatalf("status bar missing from View: %q", out)
	}
	if !strings.Contains(out, "view:") {
		t.Fatalf("status bar missing view name: %q", out)
	}
}

func TestModel_SSEEventForwardedToCurrentView(t *testing.T) {
	m := NewModel(nil, nil, "test-repo")
	rec := &recordingView{}
	m = m.WithView(ViewItems, rec)
	m = m.SetCurrent(ViewItems)

	updated, _ := m.Update(client.Event{Kind: "item_changed", Payload: []byte(`{"x":1}`)})
	_ = updated
	if len(rec.received) != 1 {
		t.Fatalf("recordingView received %d events want 1", len(rec.received))
	}
	if ev, ok := rec.received[0].(client.Event); !ok || ev.Kind != "item_changed" {
		t.Fatalf("expected client.Event{item_changed}, got %T %+v", rec.received[0], rec.received[0])
	}
}

type recordingView struct {
	received []tea.Msg
}

func (r *recordingView) Init() tea.Cmd { return nil }
func (r *recordingView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	r.received = append(r.received, msg)
	return r, nil
}
func (r *recordingView) View() string { return "recording" }
