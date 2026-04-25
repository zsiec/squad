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
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the squad dashboard (HTTP + SSE)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if code := runServeCtx(ctx, port, bind, squadDir, cmd.OutOrStdout()); code != 0 {
				return fmt.Errorf("serve exited with code %d", code)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 7777, "TCP port to bind")
	cmd.Flags().StringVar(&bind, "bind", "127.0.0.1", "interface to bind (default localhost only)")
	cmd.Flags().StringVar(&squadDir, "squad-dir", ".squad", "squad directory containing items/ and done/")
	return cmd
}

func runServeCtx(ctx context.Context, port int, bind, squadDir string, out interface{ Write([]byte) (int, error) }) int {
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
		Host: bind, Port: port, SquadDir: squadDir, RepoID: repoID,
	})

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
	}()

	fmt.Fprintf(out, "Squad dashboard: http://%s\n", addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	return 0
}
