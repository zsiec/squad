package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/hygiene"
	"github.com/zsiec/squad/internal/store"
)

const versionString = "0.1.0-dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "squad",
		Short:         "Project-management framework for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd())
	root.AddCommand(newRegisterCmd())
	root.AddCommand(newWhoamiCmd())
	root.AddCommand(newNewCmd())
	root.AddCommand(newNextCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newClaimCmd())
	root.AddCommand(newReleaseCmd())
	root.AddCommand(newDoneCmd())
	root.AddCommand(newBlockedCmd())
	root.AddCommand(newForceReleaseCmd())
	root.AddCommand(newReassignCmd())
	root.AddCommand(newSayCmd())
	root.AddCommand(newAskCmd())
	root.AddCommand(newAnswerCmd())
	root.AddCommand(newChattyCmd(thinkingVerb))
	root.AddCommand(newChattyCmd(stuckVerb))
	root.AddCommand(newChattyCmd(milestoneVerb))
	root.AddCommand(newChattyCmd(fyiVerb))
	root.AddCommand(newKnockCmd())
	root.AddCommand(newHandoffCmd())
	root.AddCommand(newReviewRequestCmd())
	root.AddCommand(newProgressCmd())
	root.AddCommand(newTickCmd())
	root.AddCommand(newWhoCmd())
	root.AddCommand(newTailCmd())
	root.AddCommand(newHistoryCmd())
	root.AddCommand(newTouchCmd())
	root.AddCommand(newUntouchCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newArchiveCmd())
	root.AddCommand(newDumpStatusCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newWorkspaceCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newInstallPluginCmd())
	root.AddCommand(newInstallHooksCmd())
	root.AddCommand(newPRCmd())
	root.AddCommand(newPRLinkCmd())
	root.AddCommand(newPRCloseCmd())
	root.PersistentPostRunE = postRunHygiene
	return root
}

// postRunHygiene fires the hygiene runner after each successful command,
// debounced via ~/.squad/hygiene.lock. Best-effort: any error (missing home
// dir, no repo, write failure) is silently swallowed so user commands never
// fail because hygiene tripped. Disable entirely with SQUAD_NO_HYGIENE=1.
func postRunHygiene(cmd *cobra.Command, args []string) error {
	if os.Getenv("SQUAD_NO_HYGIENE") != "" {
		return nil
	}
	if err := store.EnsureHome(); err != nil {
		return nil
	}
	home, err := store.Home()
	if err != nil {
		return nil
	}
	lock := filepath.Join(home, "hygiene.lock")
	r := hygiene.NewRunner(lock, 10*time.Second)
	_ = r.RunIfDue(context.Background(), func(ctx context.Context) error {
		bc, err := bootClaimContext(ctx)
		if err != nil {
			return nil
		}
		defer bc.Close()
		adapter := itemsHygieneAdapter{squadDir: filepath.Dir(bc.itemsDir)}
		sw := hygiene.New(bc.db, bc.repoID, adapter)
		_ = sw.MarkStaleAgents(ctx)
		_, _ = sw.ReclaimStale(ctx)
		return nil
	})
	return nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the squad version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), versionString)
			return nil
		},
	}
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
