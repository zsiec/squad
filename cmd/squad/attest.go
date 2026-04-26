package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/repo"
)

func newAttestCmd() *cobra.Command {
	var item, kind, command, findingsFile, reviewerAgent string
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

			if k == attest.KindReview {
				rec, err := recordReviewAttestation(ctx, L, recordReviewArgs{
					ItemID:        item,
					AgentID:       bc.agentID,
					ReviewerAgent: reviewerAgent,
					FindingsFile:  findingsFile,
					AttDir:        attDir,
				})
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "attest review %s exit=%d hash=%s\n", item, rec.ExitCode, rec.OutputHash)
				return nil
			}

			if command == "" {
				return fmt.Errorf("--command is required for kind=%s", kind)
			}

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
				bc.Close()
				os.Exit(rec.ExitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&item, "item", "", "item id (required)")
	cmd.Flags().StringVar(&kind, "kind", "", "test|lint|typecheck|build|review|manual")
	cmd.Flags().StringVar(&command, "command", "", "shell command to run and capture")
	cmd.Flags().StringVar(&findingsFile, "findings-file", "", "review findings file (kind=review only)")
	cmd.Flags().StringVar(&reviewerAgent, "reviewer-agent", "", "reviewer agent id (kind=review only)")
	return cmd
}

type recordReviewArgs struct {
	ItemID        string
	AgentID       string
	ReviewerAgent string
	FindingsFile  string
	AttDir        string
}

func recordReviewAttestation(ctx context.Context, L *attest.Ledger, a recordReviewArgs) (attest.Record, error) {
	if a.FindingsFile == "" {
		return attest.Record{}, fmt.Errorf("--findings-file is required for kind=review")
	}
	if a.ReviewerAgent == "" {
		return attest.Record{}, fmt.Errorf("--reviewer-agent is required for kind=review")
	}
	body, err := os.ReadFile(a.FindingsFile)
	if err != nil {
		return attest.Record{}, fmt.Errorf("read findings: %w", err)
	}
	exit := parseReviewExit(body)
	hash := L.Hash(body)
	if err := os.MkdirAll(a.AttDir, 0o755); err != nil {
		return attest.Record{}, err
	}
	out := filepath.Join(a.AttDir, hash+".txt")
	if err := os.WriteFile(out, body, 0o644); err != nil {
		return attest.Record{}, err
	}
	rec := attest.Record{
		ItemID:     a.ItemID,
		Kind:       attest.KindReview,
		Command:    "review by " + a.ReviewerAgent,
		ExitCode:   exit,
		OutputHash: hash,
		OutputPath: out,
		AgentID:    a.AgentID,
	}
	id, err := L.Insert(ctx, rec)
	if err != nil {
		return attest.Record{}, err
	}
	rec.ID = id
	return rec, nil
}

func parseReviewExit(body []byte) int {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "status:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(line, "status:"))
		if val == "blocking" {
			return 1
		}
		return 0
	}
	return 0
}
