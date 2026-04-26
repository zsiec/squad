package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/attest"
	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/items"
)

// EvidenceMissingError signals that the item declares evidence_required and
// at least one of those kinds has no successful attestation. The wrapper
// renders the existing user-facing message; MCP callers can inspect Missing
// directly.
type EvidenceMissingError struct {
	ItemID  string
	Missing []attest.Kind
}

func (e *EvidenceMissingError) Error() string {
	return fmt.Sprintf("%s: evidence_required not satisfied. Missing kinds: %s", e.ItemID, joinKinds(e.Missing))
}

type DoneArgs struct {
	DB       *sql.DB `json:"-"`
	RepoID   string  `json:"repo_id"`
	AgentID  string  `json:"agent_id"`
	ItemID   string  `json:"item_id"`
	Summary  string  `json:"summary,omitempty"`
	ItemsDir string  `json:"items_dir,omitempty"`
	DoneDir  string  `json:"done_dir,omitempty"`
	RepoRoot string  `json:"repo_root,omitempty"`
	Force    bool    `json:"force,omitempty"`
	// Now is an optional clock for deterministic tests; nil means time.Now.
	// The override-record body and the underlying ledger insert both honour
	// this clock when set.
	Now func() time.Time `json:"-"`
}

type DoneResult struct {
	ItemID        string        `json:"item_id"`
	Summary       string        `json:"summary,omitempty"`
	ClosedAt      int64         `json:"closed_at"`
	ForceOverride bool          `json:"force_override,omitempty"`
	BypassedKinds []attest.Kind `json:"bypassed_kinds,omitempty"`
}

// Done releases the claim and rewrites the item file in done state. The
// verification gating that decides whether close-out is allowed lives in
// the cobra wrapper rather than here so it can stream gate progress to the
// user's terminal in real time. Pure callers (CLI alternatives, tests) can
// call runVerification themselves before invoking Done.
func Done(ctx context.Context, args DoneArgs) (*DoneResult, error) {
	clock := args.Now
	if clock == nil {
		clock = time.Now
	}

	itemPath := ""
	if args.ItemsDir != "" {
		itemPath = findItemPath(args.ItemsDir, args.ItemID)
	}
	if itemPath == "" {
		return nil, fmt.Errorf("%w: %s in %s", ErrItemNotFound, args.ItemID, args.ItemsDir)
	}

	parsed, perr := items.Parse(itemPath)
	if perr != nil {
		return nil, perr
	}

	required := requiredKinds(parsed.EvidenceRequired)
	var bypassed []attest.Kind
	if len(required) > 0 {
		L := attest.New(args.DB, args.RepoID, clock)
		missing, mErr := L.MissingKinds(ctx, args.ItemID, required)
		if mErr != nil {
			return nil, mErr
		}
		if len(missing) > 0 {
			if !args.Force {
				return nil, &EvidenceMissingError{ItemID: args.ItemID, Missing: missing}
			}
			if err := recordForceOverride(ctx, L, args.RepoRoot, args.AgentID, args.ItemID, missing, clock); err != nil {
				return nil, err
			}
			bypassed = missing
		}
	}

	store := claims.New(args.DB, args.RepoID, clock)
	if err := store.Done(ctx, args.ItemID, args.AgentID, claims.DoneOpts{
		Summary:  args.Summary,
		ItemPath: itemPath,
		DoneDir:  args.DoneDir,
	}); err != nil {
		return nil, err
	}
	return &DoneResult{
		ItemID:        args.ItemID,
		Summary:       args.Summary,
		ClosedAt:      clock().Unix(),
		ForceOverride: len(bypassed) > 0,
		BypassedKinds: bypassed,
	}, nil
}

func newDoneCmd() *cobra.Command {
	var summary string
	var skipVerify bool
	var force bool
	cmd := &cobra.Command{
		Use:   "done <ITEM-ID>",
		Short: "Mark an item done: run pre-commit verification, release claim, rewrite frontmatter, move to .squad/done/",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()

			repoRoot, derr := discoverRepoRoot()
			if derr != nil {
				return derr
			}
			if !skipVerify {
				cfg, _ := config.Load(repoRoot)
				if code := runVerification(cfg.Verification.PreCommit, repoRoot, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
					os.Exit(code)
				}
			}

			res, err := Done(ctx, DoneArgs{
				DB:       bc.db,
				RepoID:   bc.repoID,
				AgentID:  bc.agentID,
				ItemID:   itemID,
				Summary:  summary,
				ItemsDir: bc.itemsDir,
				DoneDir:  bc.doneDir,
				RepoRoot: repoRoot,
				Force:    force,
			})
			if err == nil {
				if res.ForceOverride {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: --force override recorded for %s (bypassed: %s)\n",
						res.ItemID, joinKinds(res.BypassedKinds))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "done %s\n", res.ItemID)
				return nil
			}
			var miss *EvidenceMissingError
			if errors.As(err, &miss) {
				return fmt.Errorf(
					"%s: evidence_required not satisfied (missing kinds: %s) — "+
						"run `squad attest %s --kind <kind> --command \"...\"` for each, "+
						"or pass --force to override (the override is recorded as a manual attestation)",
					miss.ItemID, joinKinds(miss.Missing), miss.ItemID)
			}
			if errors.Is(err, ErrItemNotFound) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"%s: no parseable item file found in %s. Fix the file (squad doctor will list malformed_item findings) or move it back from done/, then re-run squad done.\n",
					itemID, bc.itemsDir)
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrNotYours) {
				fmt.Fprintln(cmd.ErrOrStderr(), "not your claim")
				os.Exit(1)
			}
			if errors.Is(err, claims.ErrNotClaimed) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no active claim on %s\n", itemID)
				os.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&summary, "summary", "", "one-line summary appended to the done message")
	cmd.Flags().BoolVar(&skipVerify, "skip-verify", false, "skip the verification.pre_commit gates from .squad/config.yaml")
	cmd.Flags().BoolVar(&force, "force", false, "override missing evidence_required (records a manual attestation logging the override)")
	return cmd
}

func requiredKinds(raw []string) []attest.Kind {
	out := make([]attest.Kind, 0, len(raw))
	for _, r := range raw {
		k := attest.Kind(strings.TrimSpace(r))
		if k.Valid() {
			out = append(out, k)
		}
	}
	return out
}

func joinKinds(ks []attest.Kind) string {
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}

func recordForceOverride(ctx context.Context, L *attest.Ledger, repoRoot, agentID, itemID string, missing []attest.Kind, clock func() time.Time) error {
	body := fmt.Sprintf(
		"FORCE OVERRIDE for %s\nbypassed_kinds: %s\nagent: %s\nat: %s\n",
		itemID, joinKinds(missing), agentID, clock().UTC().Format(time.RFC3339),
	)
	attDir := filepath.Join(repoRoot, ".squad", "attestations")
	hash := L.Hash([]byte(body))
	if err := os.MkdirAll(attDir, 0o755); err != nil {
		return err
	}
	out := filepath.Join(attDir, hash+".txt")
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return err
	}
	if _, err := L.Insert(ctx, attest.Record{
		ItemID:     itemID,
		Kind:       attest.KindManual,
		Command:    "force override of evidence_required",
		ExitCode:   0,
		OutputHash: hash,
		OutputPath: out,
		AgentID:    agentID,
	}); err != nil {
		return err
	}
	return nil
}

// runVerification executes each verification.pre_commit entry in cwd-relative
// terms (rooted at the repo). Each entry's evidence regex is grepped from the
// command's combined output; if non-empty and not matched, the gate fails.
// Returns 0 on success, non-zero on first failure (subsequent gates are
// skipped to keep the user signal precise).
func runVerification(gates []config.VerificationCmd, repoRoot string, stdout, stderr io.Writer) int {
	if len(gates) == 0 {
		return 0
	}
	fmt.Fprintf(stdout, "running %d verification gate(s) before close...\n", len(gates))
	for _, g := range gates {
		if g.Cmd == "" {
			continue
		}
		fmt.Fprintf(stdout, "  $ %s\n", g.Cmd)
		c := exec.Command("sh", "-c", g.Cmd)
		c.Dir = repoRoot
		out, err := c.CombinedOutput()
		if err != nil {
			fmt.Fprintf(stderr, "verification failed: %s\nexit: %v\noutput:\n%s\n", g.Cmd, err, string(out))
			fmt.Fprintf(stderr, "(use --skip-verify to bypass; fix the failure first if at all possible)\n")
			return 1
		}
		if g.Evidence != "" {
			re, err := regexp.Compile(g.Evidence)
			if err != nil {
				fmt.Fprintf(stderr, "verification: bad evidence regex %q: %v\n", g.Evidence, err)
				return 1
			}
			if !re.Match(out) {
				fmt.Fprintf(stderr, "verification: command succeeded but evidence pattern %q not found in output:\n%s\n",
					g.Evidence, string(out))
				return 1
			}
		}
		fmt.Fprintln(stdout, "    ok")
	}
	return 0
}
