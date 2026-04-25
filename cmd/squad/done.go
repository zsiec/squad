package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/claims"
	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/repo"
)

func newDoneCmd() *cobra.Command {
	var summary string
	var skipVerify bool
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

			if !skipVerify {
				wd, _ := os.Getwd()
				if root, derr := repo.Discover(wd); derr == nil {
					cfg, _ := config.Load(root)
					if code := runVerification(cfg.Verification.PreCommit, root, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
						os.Exit(code)
					}
				}
			}

			itemPath := findItemPath(bc.itemsDir, itemID)
			if itemPath == "" {
				// QA r6 H #6: Done() previously accepted an empty ItemPath and
				// committed a release without rewriting/moving the file —
				// users saw 'done <ID>' on stdout but the file stayed in
				// items/ untouched. Refuse upfront so the user knows to fix
				// the underlying parse error first (or the missing file).
				fmt.Fprintf(cmd.ErrOrStderr(),
					"%s: no parseable item file found in %s. Fix the file (squad doctor will list malformed_item findings) or move it back from done/, then re-run squad done.\n",
					itemID, bc.itemsDir)
				os.Exit(1)
			}
			err = bc.store.Done(ctx, itemID, bc.agentID, claims.DoneOpts{
				Summary:  summary,
				ItemPath: itemPath,
				DoneDir:  bc.doneDir,
			})
			if err == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "done %s\n", itemID)
				return nil
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
	return cmd
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
