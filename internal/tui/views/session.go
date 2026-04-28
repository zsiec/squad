package views

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/zsiec/squad/internal/tui/client"
)

var sessionKinds = []string{"say", "ask", "fyi"}

type SessionExitMsg struct{}

type sessionLoadedMsg struct {
	msgs []client.Message
	me   string
	err  error
}

type sessionSendOKMsg struct{ LocalID int64 }
type sessionSendFailedMsg struct {
	LocalID int64
	Err     error
}

type pendingMsg struct {
	LocalID int64
	Body    string
	Kind    string
	Failed  bool
}

type SessionModel struct {
	client     *client.Client
	target     string
	me         string
	msgs       []client.Message
	err        error
	composer   string
	kind       string
	cursor     int
	autoScroll bool
	width      int
	height     int
	pending    []pendingMsg
	renderer   *glamour.TermRenderer
}

func NewSession(c *client.Client, target string) SessionModel {
	r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(80))
	return SessionModel{
		client:     c,
		target:     target,
		kind:       "say",
		autoScroll: true,
		width:      80,
		height:     24,
		renderer:   r,
	}
}

func (m SessionModel) Kind() string { return m.kind }

func (m SessionModel) Init() tea.Cmd {
	c := m.client
	target := m.target
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		who, _ := c.Whoami(ctx)
		all, err := c.Messages(ctx, &client.MessagesOpts{Limit: 300})
		for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
			all[i], all[j] = all[j], all[i]
		}
		me := who.AgentID
		filtered := filterForSession(all, target, me)
		return sessionLoadedMsg{msgs: filtered, me: me, err: err}
	}
}

func filterForSession(all []client.Message, target, me string) []client.Message {
	out := make([]client.Message, 0, len(all))
	tagT := "@" + target
	tagM := "@" + me
	for _, msg := range all {
		switch {
		case msg.Thread == target,
			msg.AgentID == target,
			strings.Contains(msg.Body, tagT),
			me != "" && strings.Contains(msg.Body, tagM):
			out = append(out, msg)
		}
	}
	return out
}

func (m SessionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionLoadedMsg:
		m.msgs = msg.msgs
		m.me = msg.me
		m.err = msg.err
		if m.autoScroll && len(m.msgs) > 0 {
			m.cursor = len(m.msgs) - 1
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(msg.Width))
		m.renderer = r
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlK:
			m.kind = nextKind(m.kind)
			return m, nil
		case tea.KeyEsc:
			if m.composer == "" {
				return m, func() tea.Msg { return SessionExitMsg{} }
			}
			m.composer = ""
			return m, nil
		case tea.KeyEnter:
			if strings.TrimSpace(m.composer) == "" {
				return m, nil
			}
			body := m.composer
			localID := time.Now().UnixNano()
			m.pending = append(m.pending, pendingMsg{LocalID: localID, Body: body, Kind: m.kind})
			m.composer = ""
			return m, m.send(localID, body, m.kind)
		case tea.KeyBackspace:
			if len(m.composer) > 0 {
				m.composer = m.composer[:len(m.composer)-1]
			}
			return m, nil
		case tea.KeyRunes:
			m.composer += string(msg.Runes)
			return m, nil
		}
		return m, nil

	case client.Event:
		if msg.Kind == "message" {
			var p struct {
				ID      int64  `json:"id"`
				TS      int64  `json:"ts"`
				AgentID string `json:"agent_id"`
				Thread  string `json:"thread"`
				Kind    string `json:"kind"`
				Body    string `json:"body"`
			}
			if err := json.Unmarshal(msg.Payload, &p); err == nil {
				if qualifies(p.Thread, p.AgentID, p.Body, m.target, m.me) {
					m.msgs = append(m.msgs, client.Message{
						ID: p.ID, TS: p.TS, AgentID: p.AgentID, Thread: p.Thread, Kind: p.Kind, Body: p.Body,
					})
					if m.autoScroll {
						m.cursor = len(m.msgs) - 1
					}
				}
			}
		}
		return m, nil

	case sessionSendOKMsg:
		m.pending = removePending(m.pending, msg.LocalID)
		return m, nil

	case sessionSendFailedMsg:
		for i := range m.pending {
			if m.pending[i].LocalID == msg.LocalID {
				m.pending[i].Failed = true
			}
		}
		return m, nil

	case RefreshMsg:
		return m, m.Init()
	}

	return m, nil
}

func qualifies(thread, agentID, body, target, me string) bool {
	if thread == target || agentID == target {
		return true
	}
	if strings.Contains(body, "@"+target) {
		return true
	}
	if me != "" && strings.Contains(body, "@"+me) {
		return true
	}
	return false
}

func removePending(p []pendingMsg, localID int64) []pendingMsg {
	out := p[:0]
	for _, x := range p {
		if x.LocalID != localID {
			out = append(out, x)
		}
	}
	return out
}

func nextKind(cur string) string {
	for i, k := range sessionKinds {
		if k == cur {
			return sessionKinds[(i+1)%len(sessionKinds)]
		}
	}
	return sessionKinds[0]
}

func (m SessionModel) send(localID int64, body, kind string) tea.Cmd {
	c := m.client
	target := m.target
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := c.PostMessage(ctx, &client.PostMessageReq{
			Thread:   "global",
			Body:     body,
			Kind:     kind,
			Mentions: []string{target},
		})
		if err != nil {
			return sessionSendFailedMsg{LocalID: localID, Err: err}
		}
		return sessionSendOKMsg{LocalID: localID}
	}
}

func (m SessionModel) View() string {
	header := fmt.Sprintf("[session: %s — me: %s]", m.target, m.me)

	var b strings.Builder
	for _, msg := range m.msgs {
		ts := ""
		if msg.TS > 0 {
			ts = time.Unix(msg.TS, 0).Format("15:04:05")
		}
		fmt.Fprintf(&b, "[%s] %s @ %s (%s)\n", ts, msg.AgentID, msg.Thread, msg.Kind)
		body := msg.Body
		if m.renderer != nil {
			if r, err := m.renderer.Render(body); err == nil {
				body = strings.TrimRight(r, "\n")
			}
		}
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	for _, p := range m.pending {
		prefix := "(sending) "
		if p.Failed {
			prefix = "✗ failed: "
		}
		b.WriteString(prefix)
		b.WriteString(p.Body)
		b.WriteString("\n\n")
	}
	transcript := b.String()

	composer := fmt.Sprintf("kind:%s  | %s_ \n  Ctrl-K cycle kind   Enter send   Esc back", m.kind, m.composer)

	return lipgloss.JoinVertical(lipgloss.Left, header, transcript, composer)
}
