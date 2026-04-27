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
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/identity"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/server"
	"github.com/zsiec/squad/internal/store"
	"github.com/zsiec/squad/internal/tui/daemon"
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
			if installFlag, _ := cmd.Flags().GetBool("install-service"); installFlag {
				return runInstallService(cmd)
			}
			if uninstallFlag, _ := cmd.Flags().GetBool("uninstall-service"); uninstallFlag {
				return daemon.New().Uninstall()
			}
			if reinstallFlag, _ := cmd.Flags().GetBool("reinstall-service"); reinstallFlag {
				return runReinstallService(cmd)
			}
			if statusFlag, _ := cmd.Flags().GetBool("service-status"); statusFlag {
				return runServiceStatus(cmd)
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			// runServeCtx returns the intended exit code (4 for startup
			// failures). Cobra's default error path would replace any non-zero
			// with 2, so call os.Exit directly to preserve the signal scripts
			// can key on.
			if code := runServeCtx(ctx, port, bind, squadDir, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&port, "port", 7777, "TCP port to bind")
	cmd.Flags().StringVar(&bind, "bind", "127.0.0.1", "interface to bind (default localhost only)")
	cmd.Flags().StringVar(&squadDir, "squad-dir", ".squad", "squad directory containing items/ and done/")
	cmd.Flags().Bool("install-service", false, "install squad serve as a system service (launchd / systemd-user)")
	cmd.Flags().Bool("uninstall-service", false, "uninstall the system service")
	cmd.Flags().Bool("reinstall-service", false, "reinstall with current binary path")
	cmd.Flags().Bool("service-status", false, "print service status")
	return cmd
}

func runInstallService(cmd *cobra.Command) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve squad binary: %w", err)
	}
	if err := installServiceFlow(home, binary, daemon.New()); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "squad serve installed")
	return nil
}

func runReinstallService(cmd *cobra.Command) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	binary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve squad binary: %w", err)
	}
	mgr := daemon.New()
	if err := mgr.Uninstall(); err != nil {
		return err
	}
	if err := installServiceFlow(home, binary, mgr); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "squad serve reinstalled")
	return nil
}

func runServiceStatus(cmd *cobra.Command) error {
	s, err := daemon.New().Status()
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "installed: %v\nrunning:   %v\n", s.Installed, s.Running)
	if s.PID != 0 {
		fmt.Fprintf(out, "pid:       %d\n", s.PID)
	}
	return nil
}

func runServeCtx(ctx context.Context, port int, bind, squadDir string, out interface{ Write([]byte) (int, error) }) int {
	// Reject host:port forms early — users instinctively try
	// `--bind 127.0.0.1:8080` and the resulting startup error would otherwise
	// be a cryptic net.Listen failure.
	if strings.Contains(bind, ":") && !strings.Contains(bind, "::") && net.ParseIP(bind) == nil {
		fmt.Fprintf(os.Stderr,
			"squad serve: --bind takes only a host or IP, not a host:port pair (got %q).\n"+
				"  use --port for the port; e.g. --bind 127.0.0.1 --port 8080.\n", bind)
		return 4
	}
	db, err := store.OpenDefault()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	defer db.Close()

	// Repo discovery is best-effort. Single-repo mode lights up when the
	// daemon's cwd is inside a squad repo (foreground `squad serve` from a
	// repo dir). If discovery fails — the launchd / systemd-user case where
	// the daemon's cwd is `/` or the user's home — fall through to
	// workspace mode: cfg.RepoID stays empty, the server enumerates repos
	// via the global DB on each request and aggregates. The default `.squad`
	// path becomes meaningless in that mode and is reset to "" so handlers
	// can branch off the empty value rather than a misleading relative.
	repoID := ""
	repoRoot := ""
	if wd, werr := os.Getwd(); werr == nil {
		if root, rerr := repo.Discover(wd); rerr == nil {
			repoRoot = root
			if id, ierr := repo.IDFor(root); ierr == nil {
				repoID = id
				if squadDir == ".squad" {
					squadDir = filepath.Join(root, ".squad")
				}
			}
		}
	}
	if repoID == "" && squadDir == ".squad" {
		squadDir = ""
	}

	binaryPath, _ := os.Executable()
	s := server.New(db, repoID, server.Config{
		Host: bind, Port: port,
		SquadDir: squadDir, RepoID: repoID,
		LearningsRoot: repoRoot,
		Version:       versionString,
		BinaryPath:    binaryPath,
	})
	// Derive a default acting identity for the daemon. SPA sessions hit
	// /api/whoami at boot to populate X-Squad-Agent for subsequent
	// mutations; without a default the SPA cached an empty id and every
	// chat compose / claim / etc. failed with "X-Squad-Agent header
	// required". TUI/CLI clients still send their own header and override
	// this default.
	if id, err := identity.AgentID(); err == nil && id != "" {
		s.WithCallerAgent(id)
	}
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
