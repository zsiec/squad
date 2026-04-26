package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTable_DownMovesCursor(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "ID", Width: 8}},
		[][]string{{"a"}, {"b"}, {"c"}},
	)
	if m.Cursor() != 0 {
		t.Fatalf("initial cursor=%d", m.Cursor())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Cursor() != 1 {
		t.Fatalf("after down cursor=%d", m.Cursor())
	}
}

func TestTable_VimDownMovesCursor(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "ID", Width: 8}},
		[][]string{{"a"}, {"b"}, {"c"}},
	)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.Cursor() != 1 {
		t.Fatalf("after j cursor=%d", m.Cursor())
	}
}

func TestTable_FilterNarrowsRows(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "ID", Width: 8}},
		[][]string{{"abc"}, {"abd"}, {"xyz"}},
	)
	m = m.SetFilter("ab")
	if got := m.VisibleRows(); got != 2 {
		t.Fatalf("visible=%d want 2", got)
	}
	m = m.SetFilter("xy")
	if got := m.VisibleRows(); got != 1 {
		t.Fatalf("visible=%d want 1", got)
	}
	m = m.SetFilter("")
	if got := m.VisibleRows(); got != 3 {
		t.Fatalf("visible=%d want 3", got)
	}
}

func TestTable_FilterIsCaseInsensitive(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "Title", Width: 20}},
		[][]string{{"Hello"}, {"world"}, {"WORLD"}},
	)
	m = m.SetFilter("WO")
	if got := m.VisibleRows(); got != 2 {
		t.Fatalf("visible=%d want 2 (case-insensitive)", got)
	}
}

func TestTable_FilterMatchesAnyColumn(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "ID", Width: 8}, {Title: "Title", Width: 20}},
		[][]string{
			{"BUG-1", "first bug"},
			{"FEAT-2", "second feature"},
			{"BUG-3", "third bug"},
		},
	)
	m = m.SetFilter("feat")
	if got := m.VisibleRows(); got != 1 {
		t.Fatalf("visible=%d want 1 matching FEAT-", got)
	}
	m = m.SetFilter("bug")
	if got := m.VisibleRows(); got != 2 {
		t.Fatalf("visible=%d want 2 matching bug", got)
	}
}

func TestTable_SelectedRowReflectsCursor(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "ID", Width: 8}},
		[][]string{{"a"}, {"b"}, {"c"}},
	)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := m.SelectedRow()
	if len(got) != 1 || got[0] != "c" {
		t.Fatalf("selected=%v want [c]", got)
	}
}

func TestTable_SetRowsResetsCursor(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "ID", Width: 8}},
		[][]string{{"a"}, {"b"}, {"c"}},
	)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	// Replace rows; cursor should clamp into new range
	m = m.SetRows([][]string{{"x"}})
	if m.Cursor() != 0 {
		t.Fatalf("cursor after SetRows=%d want 0", m.Cursor())
	}
	if got := m.VisibleRows(); got != 1 {
		t.Fatalf("rows=%d want 1", got)
	}
}

func TestTable_ViewIncludesHeaders(t *testing.T) {
	m := NewTable(
		[]Column{{Title: "Foo", Width: 10}, {Title: "Bar", Width: 10}},
		[][]string{{"a", "b"}},
	)
	out := m.View()
	if !contains(out, "Foo") || !contains(out, "Bar") {
		t.Fatalf("View output missing headers: %q", out)
	}
}

// minimal substring helper to avoid importing strings just for tests
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
