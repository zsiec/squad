package chat

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// TailOnce writes one-shot history for a thread to w.
// thread="" or "all" means "every thread"; sinceUnix=0 means "all time".
func (c *Chat) TailOnce(ctx context.Context, w io.Writer, thread string, sinceUnix int64, kinds []string) error {
	q := `SELECT id, ts, agent_id, thread, kind, COALESCE(body, '') FROM messages WHERE ts >= ?`
	args := []any{sinceUnix}
	if thread != "" && thread != "all" {
		q += ` AND thread = ?`
		args = append(args, thread)
	}
	if len(kinds) > 0 {
		q += ` AND kind IN (?` + strings.Repeat(",?", len(kinds)-1) + `)`
		for _, k := range kinds {
			args = append(args, k)
		}
	}
	q += ` ORDER BY id`
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id, ts int64
		var agent, th, kind, body string
		if err := rows.Scan(&id, &ts, &agent, &th, &kind, &body); err != nil {
			return err
		}
		fmt.Fprintf(w, "#%-5d %s  %-10s  %-10s  %s  %s\n",
			id, time.Unix(ts, 0).Format("15:04:05"), agent, kind, ThreadLabel(th), body)
	}
	return nil
}

func ThreadLabel(t string) string {
	if t == ThreadGlobal {
		return "-> #global"
	}
	return "-> #" + t
}

// ParseSince converts a relative duration string ("30m", "2h", "7d") to a
// Unix epoch in seconds. Empty input returns 0 (= no lower bound).
func ParseSince(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix(), nil
		}
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	return time.Now().Add(-dur).Unix(), nil
}
