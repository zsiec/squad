package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/repo"
	"github.com/zsiec/squad/internal/scaffold"
	"github.com/zsiec/squad/internal/specs"
)

func newScaffoldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaffold",
		Short: "Generate documentation surfaces from current ledger state",
	}
	cmd.AddCommand(newScaffoldDocIndexCmd())
	cmd.AddCommand(newScaffoldAgentsMdCmd())
	return cmd
}

func newScaffoldDocIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doc-index",
		Short: "Write docs/specs.md and docs/epics.md from the ledger",
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			repoRoot, err := repo.Discover(wd)
			if err != nil {
				return err
			}
			squadDir := filepath.Join(repoRoot, ".squad")
			specsList, err := specs.Walk(squadDir)
			if err != nil {
				return fmt.Errorf("walk specs: %w", err)
			}
			epicsList, _, err := epics.Walk(squadDir)
			if err != nil {
				return fmt.Errorf("walk epics: %w", err)
			}
			walk, err := items.Walk(squadDir)
			if err != nil {
				return fmt.Errorf("walk items: %w", err)
			}
			all := append(append([]items.Item(nil), walk.Active...), walk.Done...)
			specsMD, epicsMD := scaffold.RenderDocIndex(specsList, epicsList, all)

			docsDir := filepath.Join(repoRoot, "docs")
			if err := os.MkdirAll(docsDir, 0o755); err != nil {
				return err
			}
			specsPath := filepath.Join(docsDir, "specs.md")
			if err := os.WriteFile(specsPath, []byte(specsMD), 0o644); err != nil {
				return err
			}
			epicsPath := filepath.Join(docsDir, "epics.md")
			if err := os.WriteFile(epicsPath, []byte(epicsMD), 0o644); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), specsPath)
			fmt.Fprintln(cmd.OutOrStdout(), epicsPath)
			return nil
		},
	}
}
