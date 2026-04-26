package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/config"
	"github.com/zsiec/squad/internal/hygiene"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/store"
)

// DoctorArgs is the input for Doctor. Mirrors bootClaimContext (db + repoID +
// items dir) so MCP can invoke the same sweep CLI uses without re-deriving.
type DoctorArgs struct {
	DB       *sql.DB `json:"-"`
	RepoID   string  `json:"-"`
	RepoRoot string  `json:"-"`
}

// DoctorFinding is the structured per-finding payload MCP callers receive.
// It carries the same fields the CLI prints in its bullet list, plus a
// severity hint so dashboards can color-code without re-parsing.
type DoctorFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

// DoctorResult is the structured response shape for MCP callers. Total is
// len(Findings) so callers can do a quick clean/dirty check without walking
// the array.
type DoctorResult struct {
	Findings []DoctorFinding `json:"findings"`
	Total    int             `json:"total"`
}

// Doctor runs the same hygiene sweep `squad doctor` runs from the CLI and
// returns a structured findings list. Hook-drift detection is intentionally
// skipped for MCP callers: it depends on knowing where the plugin is
// installed, which is a CLI-side concern.
func Doctor(ctx context.Context, args DoctorArgs) (*DoctorResult, error) {
	if args.DB == nil || args.RepoID == "" || args.RepoRoot == "" {
		return nil, errNoRepo
	}
	squadDir := filepath.Join(args.RepoRoot, ".squad")
	adapter := itemsHygieneAdapter{squadDir: squadDir}
	sw := hygiene.New(args.DB, args.RepoID, adapter)
	if cfg, err := config.Load(args.RepoRoot); err == nil && cfg.Hygiene.StaleClaimMinutes > 0 {
		sw = sw.WithStaleSeconds(int64(cfg.Hygiene.StaleClaimMinutes) * 60)
	}
	findings, err := sw.Sweep(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DoctorFinding, 0, len(findings))
	for _, f := range findings {
		out = append(out, DoctorFinding{
			Severity: severityName(f.Severity),
			Code:     f.Code,
			Message:  f.Message,
			Fix:      f.Fix,
		})
	}
	return &DoctorResult{Findings: out, Total: len(out)}, nil
}

func severityName(s hygiene.Severity) string {
	switch s {
	case hygiene.SeverityError:
		return "error"
	case hygiene.SeverityWarn:
		return "warn"
	default:
		return "info"
	}
}

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
				ID:               it.ID,
				Path:             it.Path,
				Status:           it.Status,
				Created:          it.Created,
				Updated:          it.Updated,
				References:       it.References,
				BlockedBy:        it.BlockedBy,
				EvidenceRequired: it.EvidenceRequired,
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
	var strict bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose stale claims, ghost agents, orphan touches, broken refs, and DB integrity",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			bc, err := bootClaimContext(ctx)
			if err != nil {
				return err
			}
			defer bc.Close()
			squadDir := filepath.Dir(bc.itemsDir)
			adapter := itemsHygieneAdapter{squadDir: squadDir}
			sw := hygiene.New(bc.db, bc.repoID, adapter)
			if cfg, err := config.Load(filepath.Dir(squadDir)); err == nil && cfg.Hygiene.StaleClaimMinutes > 0 {
				sw = sw.WithStaleSeconds(int64(cfg.Hygiene.StaleClaimMinutes) * 60)
			}
			findings, err := sw.Sweep(ctx)
			if err != nil {
				return err
			}
			hookFindings := checkHookDrift(cmd.OutOrStdout())
			totalFindings := len(findings) + len(hookFindings)
			if totalFindings == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "doctor: all clear")
				return nil
			}
			if len(findings) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "doctor: %d finding(s):\n", len(findings))
				for _, f := range findings {
					fmt.Fprintf(cmd.OutOrStdout(), "  - [%s] %s\n", f.Code, f.Message)
					if f.Fix != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "      fix: %s\n", f.Fix)
					}
				}
			}
			if strict {
				return fmt.Errorf("doctor: %d finding(s) — see output above", totalFindings)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false,
		"exit non-zero if any findings exist; use this in CI to fail the job")
	return cmd
}

// checkHookDrift inspects the on-disk hook scripts (the user can edit these,
// and an old install can ship hooks the newer binary has since updated)
// against the embedded canonical bytes and prints any drift. Silent skip
// when no install dir exists — squad doctor should not nag users who have
// not installed the plugin via squad.
func checkHookDrift(out io.Writer) []hygiene.HookFinding {
	dir := resolveHookInstallDir()
	if dir == "" {
		return nil
	}
	findings, err := hygiene.DetectHookDrift(dir)
	if err != nil {
		fmt.Fprintf(out, "doctor: hook drift check skipped: %v\n", err)
		return nil
	}
	if len(findings) == 0 {
		return nil
	}
	fmt.Fprintf(out, "hooks: %d installed hook script(s) differ from this binary (%s)\n", len(findings), dir)
	for _, f := range findings {
		switch f.Kind {
		case hygiene.DriftMissing:
			fmt.Fprintf(out, "  - %s: missing (run `squad install-plugin` to install)\n", f.Filename)
		default:
			fmt.Fprintf(out, "  - %s: %s (run `squad install-plugin --uninstall && squad install-plugin` to restore)\n", f.Filename, f.Kind)
		}
	}
	return findings
}

// resolveHookInstallDir picks the directory squad doctor should diff against:
//  1. ${CLAUDE_PLUGIN_ROOT}/hooks — set by Claude Code at hook-execution time.
//  2. ~/.claude/plugins/squad/hooks — where install-plugin lays the plugin.
//  3. ~/.squad/hooks — where mergeSquadHooks materializes scripts when hooks
//     are registered through settings.json.
//
// Returns "" when none exist (silent skip).
func resolveHookInstallDir() string {
	if root := os.Getenv("CLAUDE_PLUGIN_ROOT"); root != "" {
		dir := filepath.Join(root, "hooks")
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	var candidates []string
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".claude", "plugins", "squad", "hooks"))
	}
	if squadHome, err := store.Home(); err == nil {
		candidates = append(candidates, filepath.Join(squadHome, "hooks"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}
