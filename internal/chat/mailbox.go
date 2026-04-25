package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Mailbox struct {
	Agent       string          `json:"agent"`
	NowTS       int64           `json:"now_ts"`
	Knocks      []DigestMessage `json:"knocks,omitempty"`
	Mentions    []DigestMessage `json:"mentions,omitempty"`
	YourThreads []DigestMessage `json:"your_threads,omitempty"`
	Handoffs    []DigestMessage `json:"handoffs,omitempty"`
	Global      []DigestMessage `json:"global,omitempty"`
}

func (m Mailbox) Empty() bool {
	return len(m.Knocks)+len(m.Mentions)+len(m.YourThreads)+len(m.Handoffs)+len(m.Global) == 0
}

// Format renders the mailbox as the human-readable string Claude Code's
// hook framework injects via additionalContext or decision-block reason.
func (m Mailbox) Format() string {
	if m.Empty() {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[squad inbox for %s]\n", m.Agent)
	section := func(title string, msgs []DigestMessage) {
		if len(msgs) == 0 {
			return
		}
		fmt.Fprintf(&b, "\n== %s ==\n", title)
		for _, msg := range msgs {
			fmt.Fprintf(&b, "  [%s] @%s on #%s (%s): %s\n",
				time.Unix(msg.TS, 0).Format("15:04"), msg.Agent,
				msg.Thread, msg.Kind, msg.Body)
		}
	}
	section("KNOCKS (high priority)", m.Knocks)
	section("MENTIONS", m.Mentions)
	section("YOUR THREADS", m.YourThreads)
	section("HANDOFFS", m.Handoffs)
	section("GLOBAL", m.Global)
	return strings.TrimRight(b.String(), "\n")
}

// FormatJSON renders the Stop-hook decision-block envelope. Claude Code
// interprets {"decision":"block","reason":"..."} as "do not end the turn;
// feed reason in as the next user message."
func (m Mailbox) FormatJSON() string {
	body := m.Format()
	if body == "" {
		body = "(no new messages)"
	}
	out, _ := json.Marshal(struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}{Decision: "block", Reason: body})
	return string(out)
}

func (c *Chat) Mailbox(ctx context.Context, agentID string) (Mailbox, error) {
	dg, err := c.Digest(ctx, agentID)
	if err != nil {
		return Mailbox{}, err
	}
	return Mailbox{
		Agent:       dg.Agent,
		NowTS:       dg.NowTS,
		Knocks:      dg.Knocks,
		Mentions:    dg.Mentions,
		YourThreads: dg.YourThreads,
		Handoffs:    dg.Handoffs,
		Global:      dg.Global,
	}, nil
}

// MarkMailboxRead advances the per-thread read cursor for agentID up to
// the highest message id in m. Used by the listen/post-tool hooks to
// "consume" the mailbox after injecting it as additionalContext.
func (c *Chat) MarkMailboxRead(ctx context.Context, agentID string, m Mailbox) error {
	maxByThread := map[string]int64{}
	consider := func(msgs []DigestMessage) {
		for _, msg := range msgs {
			if msg.ID > maxByThread[msg.Thread] {
				maxByThread[msg.Thread] = msg.ID
			}
		}
	}
	consider(m.Knocks)
	consider(m.Mentions)
	consider(m.YourThreads)
	consider(m.Handoffs)
	consider(m.Global)
	for thread, maxID := range maxByThread {
		if _, err := c.db.ExecContext(ctx, `
			INSERT INTO reads (agent_id, thread, last_msg_id) VALUES (?, ?, ?)
			ON CONFLICT(agent_id, thread) DO UPDATE SET last_msg_id = excluded.last_msg_id
			WHERE excluded.last_msg_id > reads.last_msg_id
		`, agentID, thread, maxID); err != nil {
			return err
		}
	}
	return nil
}
