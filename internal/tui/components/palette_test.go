package components

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPalette_OpensAndClosesViaApi(t *testing.T) {
	p := NewPalette([]Command{{Name: "items"}}, "")
	if p.IsActive() {
		t.Fatal("expected inactive on construction")
	}
	p = p.Open()
	if !p.IsActive() {
		t.Fatal("expected active after Open")
	}
	p = p.Close()
	if p.IsActive() {
		t.Fatal("expected inactive after Close")
	}
}

func TestPalette_FilterNarrowsMatches(t *testing.T) {
	p := NewPalette([]Command{
		{Name: "items"}, {Name: "agents"}, {Name: "specs"}, {Name: "stats"},
	}, "")
	p = p.Open()
	// Type 's' character by character
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got := p.VisibleMatches()
	// Fuzzy on 's' should match all four (each contains an 's' or 'S')
	if len(got) != 4 {
		t.Fatalf("'s' matches=%d %v", len(got), got)
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got = p.VisibleMatches()
	// "sp" should match "specs" only
	want := []string{"specs"}
	if len(got) != len(want) || got[0].Name != want[0] {
		t.Fatalf("'sp' matches=%v want=%v", got, want)
	}
}

func TestPalette_EnterEmitsSelectedMsg(t *testing.T) {
	p := NewPalette([]Command{{Name: "items"}, {Name: "agents"}}, "")
	p = p.Open()
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// match list should be ["agents"]
	var newP Palette
	var cmd tea.Cmd
	newP, cmd = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	p = newP
	if cmd == nil {
		t.Fatal("expected non-nil cmd on Enter")
	}
	msg := cmd()
	sel, ok := msg.(PaletteSelectedMsg)
	if !ok {
		t.Fatalf("expected PaletteSelectedMsg, got %T", msg)
	}
	if sel.Command != "agents" {
		t.Fatalf("selected=%q want agents", sel.Command)
	}
	if p.IsActive() {
		t.Fatal("palette should close after selection")
	}
}

func TestPalette_EscClosesWithoutSelection(t *testing.T) {
	p := NewPalette([]Command{{Name: "items"}}, "")
	p = p.Open()
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.IsActive() {
		t.Fatal("Esc should close")
	}
	if cmd != nil {
		// Esc may or may not return nil; the important thing is no PaletteSelectedMsg.
		if msg := cmd(); msg != nil {
			if _, ok := msg.(PaletteSelectedMsg); ok {
				t.Fatal("Esc emitted PaletteSelectedMsg")
			}
		}
	}
}

func TestPalette_DownUpMovesCursor(t *testing.T) {
	p := NewPalette([]Command{{Name: "items"}, {Name: "agents"}, {Name: "stats"}}, "")
	p = p.Open()
	if p.Cursor() != 0 {
		t.Fatalf("initial cursor=%d", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.Cursor() != 1 {
		t.Fatalf("down cursor=%d", p.Cursor())
	}
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.Cursor() != 1 {
		t.Fatalf("after down-down-up cursor=%d", p.Cursor())
	}
}

func TestPalette_HistoryPersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "tui_history.txt")

	// First instance: select "items"
	p := NewPalette([]Command{{Name: "items"}, {Name: "agents"}}, histPath)
	p = p.Open()
	p, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = p
	_ = cmd()

	// Verify history file written with mode 0600
	info, err := os.Stat(histPath)
	if err != nil {
		t.Fatalf("history file not written: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode=%o want 0600", mode)
	}
	data, _ := os.ReadFile(histPath)
	if !strings.Contains(string(data), "items") {
		t.Fatalf("history missing 'items': %q", data)
	}

	// Second instance: history loaded
	p2 := NewPalette([]Command{{Name: "items"}, {Name: "agents"}}, histPath)
	if !p2.HasHistory("items") {
		t.Fatal("second instance didn't load history")
	}
}

func TestPalette_HistoryCappedAt200(t *testing.T) {
	dir := t.TempDir()
	histPath := filepath.Join(dir, "tui_history.txt")
	// Pre-write 250 lines
	var b strings.Builder
	for i := 0; i < 250; i++ {
		b.WriteString("cmd-")
		b.WriteString(strings.Repeat("x", 1)) // distinct enough
		b.WriteString("\n")
	}
	if err := os.WriteFile(histPath, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	p := NewPalette([]Command{{Name: "items"}}, histPath)
	if got := p.HistorySize(); got > 200 {
		t.Fatalf("history size=%d want ≤200", got)
	}
}
