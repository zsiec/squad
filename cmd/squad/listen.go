package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/listener"
	"github.com/zsiec/squad/internal/notify"
)

type listenArgs struct {
	Instance    string
	FallbackInt time.Duration
	MaxLifetime time.Duration
	BindAddr    string
}

func newListenCmd() *cobra.Command {
	var a listenArgs
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Block until a peer message wakes this session; emit decision-block JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			a.Instance = resolveInstance(a.Instance)
			registry := notify.NewRegistry(bc.db)
			os.Exit(runListen(ctx, bc.chat, bc.agentID, bc.repoID, registry, a, cmd.OutOrStdout()))
			return nil
		},
	}
	cmd.Flags().StringVar(&a.Instance, "instance", "", "stable instance id (default: $SQUAD_SESSION_ID or env-derived)")
	cmd.Flags().DurationVar(&a.FallbackInt, "fallback", 30*time.Second, "fallback re-check interval")
	cmd.Flags().DurationVar(&a.MaxLifetime, "max", 24*time.Hour, "exit after this duration if no wake")
	cmd.Flags().StringVar(&a.BindAddr, "bind", "127.0.0.1:0", "bind address (must be loopback)")
	return cmd
}

func runListen(ctx context.Context, c *chat.Chat, agentID, repoID string,
	registry *notify.Registry, a listenArgs, stdout io.Writer) int {

	bind := a.BindAddr
	if bind == "" {
		bind = "127.0.0.1:0"
	}
	l, err := listener.New(bind)
	if err != nil {
		fmt.Fprintf(os.Stderr, "squad listen: bind failed: %v\n", err)
		return 4
	}
	defer func() { _ = l.Close() }()

	if err := registry.Register(ctx, notify.Endpoint{
		Instance: a.Instance, RepoID: repoID, Kind: notify.KindListen, Port: l.Port(),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "squad listen: register: %v\n", err)
		return 4
	}
	defer func() { _ = registry.Unregister(context.Background(), a.Instance, notify.KindListen) }()

	wakeCtx, cancel := context.WithTimeout(ctx, a.MaxLifetime)
	defer cancel()

	pollMailbox := func() bool {
		m, err := c.Mailbox(wakeCtx, agentID)
		if err != nil || m.Empty() {
			return false
		}
		fmt.Fprintln(stdout, m.FormatJSON())
		_ = c.MarkMailboxRead(wakeCtx, agentID, m)
		return true
	}

	if pollMailbox() {
		return 2
	}

	wakeCh := make(chan struct{}, 1)
	go func() {
		_, _ = l.WaitLoop(wakeCtx, a.FallbackInt, func() {
			if pollMailbox() {
				select {
				case wakeCh <- struct{}{}:
				default:
				}
			}
		})
		select {
		case wakeCh <- struct{}{}:
		default:
		}
	}()

	select {
	case <-wakeCh:
		if pollMailbox() {
			return 2
		}
		return 0
	case <-wakeCtx.Done():
		return 0
	}
}

func resolveInstance(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, env := range []string{"SQUAD_SESSION_ID", "TERM_SESSION_ID", "ITERM_SESSION_ID", "TMUX_PANE", "WT_SESSION"} {
		if v := os.Getenv(env); v != "" {
			return "session-" + shortHash(v)
		}
	}
	return "session-" + shortHash(fmt.Sprintf("pid-%d-%d", os.Getpid(), time.Now().UnixNano()))
}

func shortHash(s string) string {
	const alphabet = "0123456789abcdef"
	var h uint32 = 2166136261
	for _, b := range []byte(s) {
		h ^= uint32(b)
		h *= 16777619
	}
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = alphabet[h&0xf]
		h >>= 4
	}
	return string(out)
}
