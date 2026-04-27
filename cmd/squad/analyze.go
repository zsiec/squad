package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/analyze"
	"github.com/zsiec/squad/internal/epics"
	"github.com/zsiec/squad/internal/items"
)

// AnalyzeArgs is the input for Analyze. Both fields are required.
type AnalyzeArgs struct {
	SquadDir string
	EpicName string
}

// AnalyzeStream mirrors analyze.Stream with explicit json tags so MCP
// callers see a stable wire shape without depending on internal package
// field names.
type AnalyzeStream struct {
	FileGlobs []string `json:"file_globs"`
	ItemIDs   []string `json:"item_ids"`
}

type AnalyzeDepEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type AnalyzeResult struct {
	Epic              string           `json:"epic"`
	Spec              string           `json:"spec"`
	Streams           []AnalyzeStream  `json:"streams"`
	Deps              []AnalyzeDepEdge `json:"deps"`
	HasCycle          bool             `json:"has_cycle"`
	ParallelismFactor float64          `json:"parallelism_factor"`
}

// ErrEpicNotFound signals a missing epic by name. MCP callers can errors.Is
// it; the cobra wrapper renders the same legacy not-found message.
var ErrEpicNotFound = errors.New("epic not found")

// Analyze loads the epic + its items from disk and runs the deterministic
// stream-decomposition. Pure of writers; the cobra wrapper handles the
// pretty-print + scaffold-write while MCP returns the raw graph.
func Analyze(_ context.Context, args AnalyzeArgs) (*AnalyzeResult, error) {
	name := strings.TrimSpace(args.EpicName)
	if name == "" {
		return nil, fmt.Errorf("epic_name required")
	}
	if args.SquadDir == "" {
		return nil, fmt.Errorf("squad_dir required")
	}
	epicList, _, err := epics.Walk(args.SquadDir)
	if err != nil {
		return nil, err
	}
	var ep epics.Epic
	for _, e := range epicList {
		if e.Name == name {
			ep = e
			break
		}
	}
	if ep.Name == "" {
		return nil, fmt.Errorf("%w: %q", ErrEpicNotFound, name)
	}

	w, err := items.Walk(args.SquadDir)
	if err != nil {
		return nil, err
	}
	var its []items.Item
	for _, group := range [][]items.Item{w.Active, w.Done} {
		for _, it := range group {
			if it.Epic == name {
				its = append(its, it)
			}
		}
	}
	a := analyze.Run(ep, its)
	return analysisToResult(a), nil
}

func analysisToResult(a analyze.Analysis) *AnalyzeResult {
	out := &AnalyzeResult{
		Epic:              a.Epic,
		Spec:              a.Spec,
		HasCycle:          a.HasCycle,
		ParallelismFactor: a.ParallelismFactor,
	}
	for _, s := range a.Streams {
		out.Streams = append(out.Streams, AnalyzeStream{
			FileGlobs: s.FileGlobs,
			ItemIDs:   s.ItemIDs,
		})
	}
	for _, d := range a.Deps {
		out.Deps = append(out.Deps, AnalyzeDepEdge{From: d.From, To: d.To})
	}
	return out
}

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
	res, err := Analyze(context.Background(), AnalyzeArgs{SquadDir: squadDir, EpicName: epicName})
	if err != nil {
		if errors.Is(err, ErrEpicNotFound) {
			fmt.Fprintf(os.Stderr, "epic %q not found\n", epicName)
			return 4
		}
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	_, code = writeScaffold(stdout, squadDir, "epics", epicName+"-analysis", renderAnalysis(res), "", false)
	return code
}

func renderAnalysis(a *AnalyzeResult) string {
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
