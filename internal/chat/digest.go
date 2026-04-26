package chat

import (
	"context"
	"strings"
)

type DigestMessage struct {
	ID       int64
	TS       int64
	Agent    string
	Thread   string
	Kind     string
	Body     string
	Priority string
}

type Digest struct {
	Agent       string
	NowTS       int64
	Knocks      []DigestMessage
	Mentions    []DigestMessage
	YourThreads []DigestMessage
	Global      []DigestMessage
	Handoffs    []DigestMessage
	LostClaims  []string
}

func (c *Chat) Digest(ctx context.Context, agentID string) (Digest, error) {
	dg := Digest{Agent: agentID, NowTS: c.nowUnix()}

	unread := func(thread string) ([]DigestMessage, error) {
		rows, err := c.db.QueryContext(ctx, `
			SELECT m.id, m.ts, m.agent_id, m.thread, m.kind, COALESCE(m.body, ''), m.priority
			FROM messages m
			LEFT JOIN reads r ON r.agent_id = ? AND r.thread = m.thread
			WHERE m.thread = ? AND m.id > COALESCE(r.last_msg_id, 0) AND m.agent_id != ?
			ORDER BY m.id
		`, agentID, thread, agentID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []DigestMessage
		for rows.Next() {
			var m DigestMessage
			if err := rows.Scan(&m.ID, &m.TS, &m.Agent, &m.Thread, &m.Kind, &m.Body, &m.Priority); err != nil {
				return nil, err
			}
			out = append(out, m)
		}
		return out, nil
	}

	globalMsgs, err := unread(ThreadGlobal)
	if err != nil {
		return dg, err
	}
	mentionTag := "@" + agentID
	for _, m := range globalMsgs {
		switch {
		case m.Kind == KindKnock && m.Priority == PriorityHigh:
			dg.Knocks = append(dg.Knocks, m)
		case m.Kind == KindHandoff:
			dg.Handoffs = append(dg.Handoffs, m)
		case strings.Contains(m.Body, mentionTag):
			dg.Mentions = append(dg.Mentions, m)
		default:
			dg.Global = append(dg.Global, m)
		}
	}

	rows, err := c.db.QueryContext(ctx,
		`SELECT item_id FROM claims WHERE repo_id = ? AND agent_id = ?`, c.repoID, agentID)
	if err != nil {
		return dg, err
	}
	var myItems []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return dg, err
		}
		myItems = append(myItems, id)
	}
	rows.Close()
	for _, item := range myItems {
		msgs, err := unread(item)
		if err != nil {
			return dg, err
		}
		dg.YourThreads = append(dg.YourThreads, msgs...)
	}
	return dg, nil
}
