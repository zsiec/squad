package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/repo"
)

func newDecomposeCmd() *cobra.Command {
	var printOnly bool
	cmd := &cobra.Command{
		Use:   "decompose <spec-name>",
		Short: "Emit (or invoke) a structured agent prompt to decompose a spec into items",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runDecompose(cmd.Context(), args[0], printOnly, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&printOnly, "print-prompt", false, "print the prompt to stdout instead of invoking claude")
	return cmd
}

const decomposePromptTmpl = `You're decomposing a squad spec into claimable items.

Spec: {{.SpecPath}}

Read the spec end-to-end. Then propose 3-7 claimable items that decompose
the spec's acceptance criteria. For each item:
  - title: ≤8 words, action-oriented
  - area: a real area tag (look at .squad/items/ for examples)
  - estimate: 30m / 1h / 4h / 1d (be honest; we'd rather have 5 1h items
    than 1 5h item)
  - risk: low / medium / high
  - acceptance criteria: ≥1 testable checkbox under '## Acceptance criteria'

For each item, call the squad_new MCP tool with parent_spec="{{.SpecName}}"
AND the fields above. Default status (captured) is correct — a human will
triage these via 'squad inbox'. Do NOT pass ready=true.

After you've called squad_new for each item, print a one-line summary:
"decomposed {{.SpecName}} into N items: ID-1, ID-2, ..."
`

var decomposeTmpl = template.Must(template.New("decompose").Parse(decomposePromptTmpl))

func runDecompose(ctx context.Context, specName string, printOnly bool, stdout, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "getwd: %v\n", err)
		return 4
	}
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintf(stderr, "find repo: %v\n", err)
		return 4
	}
	specPath := filepath.Join(root, ".squad", "specs", specName+".md")
	if _, err := os.Stat(specPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(stderr, "spec %q not found at %s; run 'squad spec-list' to see available specs\n", specName, specPath)
			return 4
		}
		fmt.Fprintf(stderr, "stat spec: %v\n", err)
		return 4
	}
	var prompt bytes.Buffer
	if err := decomposeTmpl.Execute(&prompt, map[string]string{
		"SpecPath": specPath,
		"SpecName": specName,
	}); err != nil {
		fmt.Fprintf(stderr, "render prompt: %v\n", err)
		return 4
	}
	if printOnly {
		fmt.Fprint(stdout, prompt.String())
		return 0
	}
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		fmt.Fprintln(stderr, "squad decompose: claude binary not found in PATH; falling back to --print-prompt mode")
		fmt.Fprintln(stderr, "(copy the prompt below into a Claude Code session, or pipe it into claude yourself)")
		fmt.Fprint(stdout, prompt.String())
		return 0
	}
	c := exec.CommandContext(ctx, claudePath, "--allowedTools", "squad_new")
	c.Stdin = &prompt
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		fmt.Fprintf(stderr, "claude exited: %v\n", err)
		return 4
	}
	return 0
}
