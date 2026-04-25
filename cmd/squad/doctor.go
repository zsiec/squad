package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/hygiene"
	"github.com/zsiec/squad/internal/items"
)

// itemsHygieneAdapter walks .squad/items and .squad/done and reports each
// item's id, path, status, and references for the hygiene Sweep.
type itemsHygieneAdapter struct {
	squadDir string
}

func (a itemsHygieneAdapter) List(ctx context.Context) ([]hygiene.ItemRef, error) {
	w, err := items.Walk(a.squadDir)
	if err != nil {
		return nil, err
	}
	var out []hygiene.ItemRef
	for _, group := range [][]items.Item{w.Active, w.Done} {
		for _, it := range group {
			out = append(out, hygiene.ItemRef{
				ID:         it.ID,
				Path:       it.Path,
				Status:     it.Status,
				References: it.References,
				BlockedBy:  it.BlockedBy,
			})
		}
	}
	return out, nil
}

func (a itemsHygieneAdapter) Broken(ctx context.Context) ([]hygiene.BrokenRef, error) {
	w, err := items.Walk(a.squadDir)
	if err != nil {
		return nil, err
	}
	out := make([]hygiene.BrokenRef, 0, len(w.Broken))
	for _, b := range w.Broken {
		out = append(out, hygiene.BrokenRef{Path: b.Path, Error: b.Error})
	}
	return out, nil
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose stale claims, ghost agents, orphan touches, broken refs, and DB integrity",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			adapter := itemsHygieneAdapter{squadDir: filepath.Dir(bc.itemsDir)}
			sw := hygiene.New(bc.db, bc.repoID, adapter)
			findings, err := sw.Sweep(ctx)
			if err != nil {
				return err
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "doctor: all clear")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "doctor: %d finding(s):\n", len(findings))
			for _, f := range findings {
				fmt.Fprintf(cmd.OutOrStdout(), "  - [%s] %s\n", f.Code, f.Message)
				if f.Fix != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "      fix: %s\n", f.Fix)
				}
			}
			bc.Close()
			os.Exit(1)
			return nil
		},
	}
}
