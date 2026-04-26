package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/attest"
)

// ErrInvalidKind signals that AttestArgs.Kind is not one of the
// attest.Kind enum values. Cobra wrapper renders it as the legacy
// "invalid kind" message; MCP callers can errors.Is it.
var ErrInvalidKind = errors.New("invalid attestation kind")

type AttestArgs struct {
	DB      *sql.DB `json:"-"`
	RepoID  string  `json:"repo_id"`
	AgentID string  `json:"agent_id"`

	ItemID string `json:"item_id"`
	Kind   string `json:"kind"`

	Command       string `json:"command,omitempty"`
	FindingsFile  string `json:"findings_file,omitempty"`
	ReviewerAgent string `json:"reviewer_agent,omitempty"`

	AttDir   string `json:"att_dir,omitempty"`
	RepoRoot string `json:"repo_root,omitempty"`

	Now func() time.Time `json:"-"`
}

type AttestResult struct {
	ID         int64  `json:"id"`
	ItemID     string `json:"item_id"`
	Kind       string `json:"kind"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	OutputHash string `json:"output_hash"`
	OutputPath string `json:"output_path"`
	AgentID    string `json:"agent_id"`
}

func Attest(ctx context.Context, args AttestArgs) (*AttestResult, error) {
	if args.ItemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}
	k := attest.Kind(args.Kind)
	if !k.Valid() {
		return nil, fmt.Errorf("%w %q (want test|lint|typecheck|build|review|manual)", ErrInvalidKind, args.Kind)
	}

	L := attest.New(args.DB, args.RepoID, args.Now)

	if k == attest.KindReview {
		rec, err := recordReviewAttestation(ctx, L, recordReviewArgs{
			ItemID:        args.ItemID,
			AgentID:       args.AgentID,
			ReviewerAgent: args.ReviewerAgent,
			FindingsFile:  args.FindingsFile,
			AttDir:        args.AttDir,
		})
		if err != nil {
			return nil, err
		}
		return recordToResult(rec), nil
	}

	if args.Command == "" {
		return nil, fmt.Errorf("command is required for kind=%s", args.Kind)
	}
	rec, err := L.Run(ctx, attest.RunOpts{
		ItemID:   args.ItemID,
		Kind:     k,
		Command:  args.Command,
		AgentID:  args.AgentID,
		AttDir:   args.AttDir,
		RepoRoot: args.RepoRoot,
	})
	if err != nil {
		return nil, err
	}
	return recordToResult(rec), nil
}

func recordToResult(r attest.Record) *AttestResult {
	return &AttestResult{
		ID:         r.ID,
		ItemID:     r.ItemID,
		Kind:       string(r.Kind),
		Command:    r.Command,
		ExitCode:   r.ExitCode,
		OutputHash: r.OutputHash,
		OutputPath: r.OutputPath,
		AgentID:    r.AgentID,
	}
}

func newAttestCmd() *cobra.Command {
	var item, kind, command, findingsFile, reviewerAgent string
	cmd := &cobra.Command{
		Use:   "attest",
		Short: "Record a verification artifact (test/lint/build/typecheck/manual) into the evidence ledger",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			repoRoot, err := discoverRepoRoot()
			if err != nil {
				return err
			}
			attDir := filepath.Join(repoRoot, ".squad", "attestations")

			res, err := Attest(ctx, AttestArgs{
				DB:            bc.db,
				RepoID:        bc.repoID,
				AgentID:       bc.agentID,
				ItemID:        item,
				Kind:          kind,
				Command:       command,
				FindingsFile:  findingsFile,
				ReviewerAgent: reviewerAgent,
				AttDir:        attDir,
				RepoRoot:      repoRoot,
			})
			if err != nil {
				if errors.Is(err, ErrInvalidKind) {
					return fmt.Errorf("invalid kind %q (want test|lint|typecheck|build|review|manual)", kind)
				}
				return err
			}

			if attest.Kind(res.Kind) == attest.KindReview {
				fmt.Fprintf(cmd.OutOrStdout(), "attest review %s exit=%d hash=%s\n", res.ItemID, res.ExitCode, res.OutputHash)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "attest %s %s exit=%d hash=%s\n", res.Kind, res.ItemID, res.ExitCode, res.OutputHash)
			if res.ExitCode != 0 {
				bc.Close()
				os.Exit(res.ExitCode)
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
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		if !strings.HasPrefix(trimmed, "status:") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "status:"))
		if val == "blocking" {
			return 1
		}
		return 0
	}
	return 0
}
