package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/scaffold"
)

func newInitCmd() *cobra.Command {
	var (
		yes bool
		dir string
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Scaffold a squad workspace in this repository",
		Long: `init detects the git root and primary language, asks at most three
questions (project name, ID prefixes, install Claude Code plugin), then
scaffolds AGENTS.md, .squad/config.yaml, .squad/STATUS.md, an example
item, and an idempotent block in CLAUDE.md.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, initOptions{Yes: yes, Dir: dir})
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "skip prompts and accept defaults")
	cmd.Flags().StringVar(&dir, "dir", "", "directory to init in (defaults to CWD)")
	return cmd
}

type initOptions struct {
	Yes bool
	Dir string
}

func runInit(cmd *cobra.Command, opts initOptions) error {
	dir := opts.Dir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	info, err := scaffold.DetectRepo(dir)
	if err != nil {
		return fmt.Errorf("detect repo: %w", err)
	}

	ans, err := promptAnswers(info, opts.Yes, cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return err
	}

	data := scaffold.Data{
		ProjectName:     ans.ProjectName,
		IDPrefixes:      ans.IDPrefixes,
		PrimaryLanguage: info.PrimaryLanguage,
		GitRoot:         info.GitRoot,
		Remote:          info.Remote,
		InstallPlugin:   ans.InstallPlugin,
	}

	if err := scaffold.WriteConfig(info.GitRoot, data); err != nil {
		return err
	}
	if err := scaffold.WriteStatus(info.GitRoot, data); err != nil {
		return err
	}
	if err := scaffold.WriteExampleItem(info.GitRoot, data); err != nil {
		return err
	}
	if err := scaffold.WriteAgents(info.GitRoot, data); err != nil {
		return err
	}
	if err := scaffold.WriteAgentsDeep(info.GitRoot, data); err != nil {
		return err
	}

	choice, err := resolveCLAUDEChoice(info.GitRoot, opts.Yes, cmd.InOrStdin(), cmd.OutOrStdout())
	if err != nil {
		return err
	}
	if err := scaffold.MergeCLAUDE(info.GitRoot, data, choice); err != nil {
		if errors.Is(err, scaffold.ErrMergeAborted) {
			fmt.Fprintln(cmd.OutOrStdout(), "CLAUDE.md left untouched (aborted by user).")
		} else {
			return err
		}
	}

	if err := scaffold.EnsureGitignore(info.GitRoot); err != nil {
		return err
	}

	reg, err := scaffold.BootstrapAndRegister(info.GitRoot, info.Remote, ans.ProjectName)
	if err != nil {
		return err
	}

	if ans.InstallPlugin {
		fmt.Fprintln(cmd.OutOrStdout(), "Plugin install: run `squad install-plugin` to materialize skills + commands under ~/.claude/plugins/squad/.")
		fmt.Fprintln(cmd.OutOrStdout(), "  Optional: `squad install-hooks` for the SessionStart auto-register hook (other hooks opt-in).")
	}

	printSuccess(cmd.OutOrStdout(), info, ans, reg)
	return nil
}

func resolveCLAUDEChoice(repoRoot string, yes bool, in io.Reader, out io.Writer) (scaffold.MergeChoice, error) {
	dest := filepath.Join(repoRoot, "CLAUDE.md")
	body, err := os.ReadFile(dest)
	if errors.Is(err, fs.ErrNotExist) {
		return scaffold.ChoiceBottom, nil
	}
	if err != nil {
		return 0, err
	}
	s := string(body)
	if strings.Contains(s, "<!-- squad-managed:start -->") {
		return scaffold.ChoiceBottom, nil
	}
	if strings.TrimSpace(s) == "" {
		return scaffold.ChoiceBottom, nil
	}
	if yes {
		return scaffold.ChoiceBottom, nil
	}
	fmt.Fprintln(out, "CLAUDE.md exists without squad markers. Where should the managed block go?")
	fmt.Fprintln(out, "  [t] top   [b] bottom (default)   [a] abort")
	r := bufio.NewReader(in)
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return 0, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "t", "top":
		return scaffold.ChoiceTop, nil
	case "a", "abort":
		return scaffold.ChoiceAbort, nil
	default:
		return scaffold.ChoiceBottom, nil
	}
}

func printSuccess(out io.Writer, info scaffold.RepoInfo, ans answers, reg scaffold.Registration) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Scaffolded squad in", info.GitRoot)
	fmt.Fprintln(out, "  AGENTS.md")
	fmt.Fprintln(out, "  .squad/config.yaml")
	fmt.Fprintln(out, "  .squad/STATUS.md")
	fmt.Fprintln(out, "  .squad/items/EXAMPLE-001-try-the-loop.md")
	fmt.Fprintln(out, "  CLAUDE.md (squad-managed block)")
	fmt.Fprintln(out, "  .gitignore (squad lines appended)")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Registered repo:", reg.RepoID)
	fmt.Fprintln(out, "Project name:   ", ans.ProjectName)
	fmt.Fprintln(out, "ID prefixes:    ", strings.Join(ans.IDPrefixes, ", "))
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Next:")
	fmt.Fprintln(out, "  squad next             # see what to work on")
	fmt.Fprintln(out, "  squad claim EXAMPLE-001 --intent \"trying the loop\"")
}
