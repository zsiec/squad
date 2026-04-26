package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/repo"
)

func newAttestCmd() *cobra.Command {
	var item, kind, command string
	cmd := &cobra.Command{
		Use:   "attest",
		Short: "Record a verification artifact (test/lint/build/typecheck/manual) into the evidence ledger",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if item == "" {
				return fmt.Errorf("--item is required")
			}
			k := attest.Kind(kind)
			if !k.Valid() {
				return fmt.Errorf("invalid kind %q (want test|lint|typecheck|build|review|manual)", kind)
			}
			if command == "" {
				return fmt.Errorf("--command is required for kind=%s", kind)
			}

			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			repoRoot, err := repo.Discover(wd)
			if err != nil {
				return err
			}
			attDir := filepath.Join(repoRoot, ".squad", "attestations")

			L := attest.New(bc.db, bc.repoID, nil)

			rec, err := L.Run(ctx, attest.RunOpts{
				ItemID:   item,
				Kind:     k,
				Command:  command,
				AgentID:  bc.agentID,
				AttDir:   attDir,
				RepoRoot: repoRoot,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "attest %s %s exit=%d hash=%s\n", k, item, rec.ExitCode, rec.OutputHash)
			if rec.ExitCode != 0 {
				// Mirror doctor.go: defer would be skipped by os.Exit, so close
				// the db handle explicitly first.
				bc.Close()
				os.Exit(rec.ExitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&item, "item", "", "item id (required)")
	cmd.Flags().StringVar(&kind, "kind", "", "test|lint|typecheck|build|review|manual")
	cmd.Flags().StringVar(&command, "command", "", "shell command to run and capture")
	return cmd
}
