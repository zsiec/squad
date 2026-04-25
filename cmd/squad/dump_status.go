package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/hygiene"
	"github.com/zsiec/squad/internal/items"
)

func newDumpStatusCmd() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "dump-status",
		Short: "Emit STATUS.md from current DB and items state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			squadDir := filepath.Dir(bc.itemsDir)
			w, err := items.Walk(squadDir)
			if err != nil {
				return err
			}
			statusItems := make([]hygiene.StatusItem, 0, len(w.Active)+len(w.Done))
			for _, group := range [][]items.Item{w.Active, w.Done} {
				for _, it := range group {
					statusItems = append(statusItems, hygiene.StatusItem{
						ID:       it.ID,
						Title:    it.Title,
						Priority: it.Priority,
						Status:   it.Status,
						Estimate: it.Estimate,
					})
				}
			}
			body, err := hygiene.DumpStatus(ctx, bc.db, bc.repoID, statusItems, time.Now)
			if err != nil {
				return err
			}
			if out == "-" {
				fmt.Fprint(cmd.OutOrStdout(), body)
				return nil
			}
			return os.WriteFile(out, []byte(body), 0o644)
		},
	}
	cmd.Flags().StringVar(&out, "out", "-", "output path; '-' = stdout")
	return cmd
}
