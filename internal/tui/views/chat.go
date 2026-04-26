package views

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/zsiec/squad/internal/tui/client"
)

var itemThreadRe = regexp.MustCompile(`^[A-Z][A-Z0-9]*-\d+$`)

type ChatReplyMsg struct {
	Thread string
	To     string
}

type ChatJumpToItemMsg struct {
	ItemID string
}

type chatLoadedMsg struct {
	messages []client.Message
	err      error
}

type ChatModel struct {
	client     *client.Client
	messages   []client.Message
	err        error
	cursor     int
	autoScroll bool
	width      int
	renderer   *glamour.TermRenderer
}

func NewChat(c *client.Client) ChatModel {
	r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(80))
	return ChatModel{
		client:     c,
		autoScroll: true,
		width:      80,
		renderer:   r,
	}
}

func (m ChatModel) Init() tea.Cmd { return m.fetch() }

func (m ChatModel) fetch() tea.Cmd {
	c := m.client
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		msgs, err := c.Messages(ctx, &client.MessagesOpts{Limit: 200})
		// server returns newest-first; reverse for tail rendering (oldest at top)
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
		return chatLoadedMsg{messages: msgs, err: err}
	}
}

func (m ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case chatLoadedMsg:
		m.messages = msg.messages
		m.err = msg.err
		if m.autoScroll && len(m.messages) > 0 {
			m.cursor = len(m.messages) - 1
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		r, _ := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(msg.Width))
		m.renderer = r
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.messages)-1 {
				m.cursor++
			}
			return m, nil
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.autoScroll = false
			}
			return m, nil
		case "G":
			if len(m.messages) > 0 {
				m.cursor = len(m.messages) - 1
			}
			m.autoScroll = true
			return m, nil
		case "r":
			if len(m.messages) == 0 {
				return m, nil
			}
			cur := m.messages[m.cursor]
			return m, func() tea.Msg {
				return ChatReplyMsg{Thread: cur.Thread, To: cur.AgentID}
			}
		case "i":
			if len(m.messages) == 0 {
				return m, nil
			}
			cur := m.messages[m.cursor]
			if !itemThreadRe.MatchString(cur.Thread) {
				return m, nil
			}
			return m, func() tea.Msg {
				return ChatJumpToItemMsg{ItemID: cur.Thread}
			}
		}
		return m, nil

	case client.Event:
		if msg.Kind == "message" {
			var p client.MessagePayload
			if err := json.Unmarshal(msg.Payload, &p); err == nil {
				m.messages = append(m.messages, client.Message{
					ID: p.ID, TS: p.TS, AgentID: p.AgentID,
					Thread: p.Thread, Kind: p.Kind, Body: p.Body,
				})
				if m.autoScroll {
					m.cursor = len(m.messages) - 1
				}
			}
		}
		return m, nil

	case RefreshMsg:
		return m, m.fetch()
	}

	return m, nil
}

func (m ChatModel) View() string {
	if m.err != nil {
		return "chat: error: " + m.err.Error()
	}
	if m.messages == nil {
		return "loading..."
	}
	if len(m.messages) == 0 {
		return "(no messages)"
	}
	var b strings.Builder
	for i, msg := range m.messages {
		ts := time.Unix(msg.TS, 0).Format("15:04:05")
		head := fmt.Sprintf("[%s] %s @ %s (%s)", ts, msg.AgentID, msg.Thread, msg.Kind)
		if i == m.cursor {
			head = "> " + head
		}
		body := msg.Body
		if m.renderer != nil {
			if r, err := m.renderer.Render(body); err == nil {
				body = strings.TrimRight(r, "\n")
			}
		}
		b.WriteString(head)
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n\n")
	}
	return b.String()
}
