package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
)

func newTailCmd() *cobra.Command {
	var (
		thread    string
		follow    bool
		since     string
		kindsFlag string
	)
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Print recent messages, optionally streaming new ones",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			if code := runTailBody(ctx, bc.chat, bc.db, thread, follow, since, kindsFlag, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&thread, "thread", "all", "'global', '<ITEM>', or 'all'")
	cmd.Flags().BoolVar(&follow, "follow", false, "stream new messages")
	cmd.Flags().StringVar(&since, "since", "30m", "only messages newer than duration")
	cmd.Flags().StringVar(&kindsFlag, "kind", "", "comma-separated kinds to filter")
	return cmd
}

func runTailBody(ctx context.Context, c *chat.Chat, db *sql.DB, thread string, follow bool, since, kindsFlag string, w io.Writer) int {
	sinceUnix, err := chat.ParseSince(since)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	var kinds []string
	if kindsFlag != "" {
		kinds = strings.Split(kindsFlag, ",")
	}

	if err := c.TailOnce(ctx, w, thread, sinceUnix, kinds); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	if !follow {
		return 0
	}
	return tailFollow(ctx, db, w, thread, kinds)
}

func tailFollow(ctx context.Context, db *sql.DB, w io.Writer, thread string, kinds []string) int {
	var lastID int64
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id), 0) FROM messages`).Scan(&lastID)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		q := `SELECT id, ts, agent_id, thread, kind, COALESCE(body, '') FROM messages WHERE id > ?`
		qArgs := []any{lastID}
		if thread != "" && thread != "all" {
			q += ` AND thread = ?`
			qArgs = append(qArgs, thread)
		}
		if len(kinds) > 0 {
			q += ` AND kind IN (?` + strings.Repeat(",?", len(kinds)-1) + `)`
			for _, k := range kinds {
				qArgs = append(qArgs, k)
			}
		}
		q += ` ORDER BY id`
		rows, err := db.QueryContext(ctx, q, qArgs...)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id, ts int64
			var agent, th, kind, body string
			_ = rows.Scan(&id, &ts, &agent, &th, &kind, &body)
			fmt.Fprintf(w, "%s  %-10s  %-10s  %s  %s\n",
				time.Unix(ts, 0).Format("15:04:05"), agent, kind, chat.ThreadLabel(th), body)
			if id > lastID {
				lastID = id
			}
		}
		rows.Close()
	}
	return 0
}
