// Package components hosts shared bubbletea components used by view modules.
package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zsiec/squad/internal/tui/theme"
)

// Column is the squad-side type for a table column. View modules don't
// import bubbles/table directly so the underlying widget stays swappable.
type Column struct {
	Title string
	Width int
}

// Table wraps bubbles/table.Model with squad's theme and a substring
// filter. View modules embed it inside their own bubbletea Model.
type Table struct {
	cols    []Column
	all     [][]string
	visible [][]string
	filter  string
	inner   table.Model
}

// NewTable constructs a Table from the given columns and rows.
func NewTable(cols []Column, rows [][]string) Table {
	t := Table{cols: cols, all: rows, visible: rows}
	t.inner = newInner(cols, rows)
	return t
}

func newInner(cols []Column, rows [][]string) table.Model {
	bcols := make([]table.Column, len(cols))
	for i, c := range cols {
		bcols[i] = table.Column{Title: c.Title, Width: c.Width}
	}
	brows := make([]table.Row, len(rows))
	for i, r := range rows {
		brows[i] = table.Row(r)
	}
	tm := table.New(
		table.WithColumns(bcols),
		table.WithRows(brows),
		table.WithFocused(true),
	)
	styles := table.DefaultStyles()
	styles.Header = theme.TableHeader.BorderStyle(lipgloss.NormalBorder()).BorderForeground(theme.Dim).BorderBottom(true)
	styles.Cell = theme.TableRow
	styles.Selected = theme.TableSel
	tm.SetStyles(styles)
	return tm
}

// Update forwards key/window messages to the inner table, translating
// vim-style movement keys into directional KeyMsgs first.
func (t Table) Update(msg tea.Msg) (Table, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.Type == tea.KeyRunes && len(k.Runes) == 1 {
		switch k.Runes[0] {
		case 'j':
			return t.dispatchKey(tea.KeyMsg{Type: tea.KeyDown})
		case 'k':
			return t.dispatchKey(tea.KeyMsg{Type: tea.KeyUp})
		case 'g':
			return t.dispatchKey(tea.KeyMsg{Type: tea.KeyHome})
		case 'G':
			return t.dispatchKey(tea.KeyMsg{Type: tea.KeyEnd})
		}
	}
	var cmd tea.Cmd
	t.inner, cmd = t.inner.Update(msg)
	return t, cmd
}

func (t Table) dispatchKey(msg tea.Msg) (Table, tea.Cmd) {
	var cmd tea.Cmd
	t.inner, cmd = t.inner.Update(msg)
	return t, cmd
}

// View renders the table.
func (t Table) View() string { return t.inner.View() }

// Cursor returns the index of the currently selected row in the visible
// (filtered) row set.
func (t Table) Cursor() int { return t.inner.Cursor() }

// VisibleRows returns the count of currently displayed rows after filter.
func (t Table) VisibleRows() int { return len(t.visible) }

// SelectedRow returns the cells of the currently selected row, or nil
// if there are no rows.
func (t Table) SelectedRow() []string {
	if len(t.visible) == 0 {
		return nil
	}
	r := t.inner.SelectedRow()
	if len(r) == 0 {
		return nil
	}
	return []string(r)
}

// SetFilter narrows the visible rows to those whose any cell contains
// the given substring (case-insensitive). Empty filter restores all.
func (t Table) SetFilter(q string) Table {
	t.filter = q
	if q == "" {
		t.visible = t.all
	} else {
		t.visible = filterRows(t.all, q)
	}
	t.inner = newInner(t.cols, t.visible)
	return t
}

// SetRows replaces the source row set, preserving the active filter.
// Cursor resets to 0.
func (t Table) SetRows(rows [][]string) Table {
	t.all = rows
	if t.filter == "" {
		t.visible = rows
	} else {
		t.visible = filterRows(rows, t.filter)
	}
	t.inner = newInner(t.cols, t.visible)
	return t
}

func filterRows(rows [][]string, q string) [][]string {
	needle := strings.ToLower(q)
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), needle) {
				out = append(out, row)
				break
			}
		}
	}
	return out
}
