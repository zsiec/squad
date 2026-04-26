package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/items"
)

func newStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Operational statistics (placeholder; full dashboard in R7)",
	}
	cmd.AddCommand(newStatsVerificationRateCmd())
	return cmd
}

func newStatsVerificationRateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verification-rate",
		Short: "Percentage of done items whose evidence_required is fully satisfied",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			rate, satisfied, total, err := computeVerificationRate(ctx, bc)
			if err != nil {
				return err
			}
			if total == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "verification-rate: 0/0 (no done items declare evidence_required yet)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verification-rate: %d/%d (%.0f%%)\n", satisfied, total, rate*100)
			return nil
		},
	}
}

func computeVerificationRate(ctx context.Context, bc *claimContext) (float64, int, int, error) {
	w, err := items.Walk(filepath.Dir(bc.itemsDir))
	if err != nil {
		return 0, 0, 0, err
	}
	L := attest.New(bc.db, bc.repoID, nil)
	var total, satisfied int
	for _, it := range w.Done {
		req := requiredKinds(it.EvidenceRequired)
		if len(req) == 0 {
			continue
		}
		total++
		missing, err := L.MissingKinds(ctx, it.ID, req)
		if err != nil {
			return 0, 0, 0, err
		}
		if len(missing) == 0 {
			satisfied++
		}
	}
	if total == 0 {
		return 0, 0, 0, nil
	}
	return float64(satisfied) / float64(total), satisfied, total, nil
}
