package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/analyze"
	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
)

func newAnalyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze <epic>",
		Short: "Decompose an epic's items into parallel streams",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runAnalyze(args, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

func runAnalyze(args []string, stdout io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: squad analyze <epic-name>")
		return 2
	}
	epicName := strings.TrimSpace(args[0])
	squadDir, code := discoverSquadDir()
	if code != 0 {
		return code
	}

	epicList, _, err := epics.Walk(squadDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	var ep epics.Epic
	for _, e := range epicList {
		if e.Name == epicName {
			ep = e
		}
	}
	if ep.Name == "" {
		fmt.Fprintf(os.Stderr, "epic %q not found\n", epicName)
		return 4
	}

	w, err := items.Walk(squadDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	var its []items.Item
	for _, group := range [][]items.Item{w.Active, w.Done} {
		for _, it := range group {
			if it.Epic == epicName {
				its = append(its, it)
			}
		}
	}
	a := analyze.Run(ep, its)
	_, code = writeScaffold(stdout, squadDir, "epics", epicName+"-analysis", renderAnalysis(a), "", false)
	return code
}

func renderAnalysis(a analyze.Analysis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Analysis: %s\n\n", a.Epic)
	fmt.Fprintf(&b, "Spec: %s\n", a.Spec)
	fmt.Fprintf(&b, "Streams: %d\n", len(a.Streams))
	fmt.Fprintf(&b, "Parallelism factor: %.2f\n\n", a.ParallelismFactor)
	if a.HasCycle {
		fmt.Fprintln(&b, "WARNING: dependency cycle — fix depends_on chains before dispatch.")
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "## Streams")
	for i, s := range a.Streams {
		fmt.Fprintf(&b, "\n### Stream %d\nFile globs:\n", i+1)
		if len(s.FileGlobs) == 0 {
			fmt.Fprintln(&b, "- (none)")
		}
		for _, g := range s.FileGlobs {
			fmt.Fprintf(&b, "- %s\n", g)
		}
		fmt.Fprintln(&b, "Items:")
		for _, id := range s.ItemIDs {
			fmt.Fprintf(&b, "- %s\n", id)
		}
	}
	fmt.Fprintln(&b, "\n## Dependency edges")
	if len(a.Deps) == 0 {
		fmt.Fprintln(&b, "(none)")
	}
	for _, d := range a.Deps {
		fmt.Fprintf(&b, "- %s -> %s\n", d.From, d.To)
	}
	return b.String()
}
