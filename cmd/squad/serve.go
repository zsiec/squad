package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/server"
	"github.com/zsiec/squad/internal/store"
)

func newServeCmd() *cobra.Command {
	var (
		port     int
		bind     string
		squadDir string
		token    string
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the squad dashboard (HTTP + SSE)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			tok := token
			if tok == "" {
				tok = os.Getenv("SQUAD_DASHBOARD_TOKEN")
			}
			if code := runServeCtx(ctx, port, bind, squadDir, tok, cmd.OutOrStdout()); code != 0 {
				return fmt.Errorf("serve exited with code %d", code)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 7777, "TCP port to bind")
	cmd.Flags().StringVar(&bind, "bind", "127.0.0.1", "interface to bind (default localhost only)")
	cmd.Flags().StringVar(&squadDir, "squad-dir", ".squad", "squad directory containing items/ and done/")
	cmd.Flags().StringVar(&token, "token", "", "require Bearer <token> on every request (or ?token= for SSE in browsers); falls back to $SQUAD_DASHBOARD_TOKEN")
	return cmd
}

func runServeCtx(ctx context.Context, port int, bind, squadDir, token string, out interface{ Write([]byte) (int, error) }) int {
	if !isLoopbackBind(bind) && token == "" {
		fmt.Fprintf(os.Stderr,
			"squad serve: refusing to bind %s without --token (or $SQUAD_DASHBOARD_TOKEN).\n"+
				"  unauthenticated POST /api/messages would let any host on the network impersonate any agent.\n"+
				"  pass --token <random-string> or bind to a loopback address.\n", bind)
		return 4
	}
	db, err := store.OpenDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	defer db.Close()

	repoID := ""
	if wd, werr := os.Getwd(); werr == nil {
		if root, rerr := repo.Discover(wd); rerr == nil {
			if id, ierr := repo.IDFor(root); ierr == nil {
				repoID = id
				if squadDir == ".squad" {
					squadDir = filepath.Join(root, ".squad")
				}
			}
		}
	}

	s := server.New(db, repoID, server.Config{
		Host: bind, Port: port, SquadDir: squadDir, RepoID: repoID, Token: token,
	})
	defer s.Close()

	addr := net.JoinHostPort(bind, strconv.Itoa(port))
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		s.Close()
	}()

	fmt.Fprintf(out, "Squad dashboard: http://%s\n", addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}

// isLoopbackBind reports whether the user's --bind value targets only the
// local host. The unauthenticated-impersonation gate uses this to decide
// whether to require a token. We accept the canonical loopback addresses
// (IPv4 + IPv6) and "localhost"; anything else (0.0.0.0, an interface IP,
// or a hostname) is treated as network-exposed.
func isLoopbackBind(bind string) bool {
	switch bind {
	case "127.0.0.1", "::1", "localhost":
		return true
	}
	ip := net.ParseIP(bind)
	if ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}
