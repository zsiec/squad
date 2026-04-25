package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zsiec/squad/internal/repo"
)

func newSpecNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "spec-new <name> <title>",
		Short: "Create a new spec scaffold under .squad/specs/<name>.md",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runSpecNew(args, cmd.OutOrStdout()); code != 0 {
				os.Exit(code)
			}
			return nil
		},
	}
}

const specStub = `---
title: %s
motivation: |
  Why does this matter? 1–3 sentences.
acceptance:
  - first concrete acceptance criterion
  - second concrete acceptance criterion
non_goals:
  - explicit non-goal one
integration:
  - which areas of the codebase this touches
---

## Background
`

var slugSafe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func runSpecNew(args []string, stdout io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: squad spec-new <name> \"<title>\"")
		return 2
	}
	name := strings.TrimSpace(args[0])
	title := strings.TrimSpace(strings.Join(args[1:], " "))
	if !slugSafe.MatchString(name) {
		fmt.Fprintf(os.Stderr, "spec name %q must be kebab-case\n", name)
		return 4
	}
	if title == "" {
		fmt.Fprintln(os.Stderr, "spec title required")
		return 4
	}
	wd, _ := os.Getwd()
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo: %v\n", err)
		return 4
	}
	dir := filepath.Join(root, ".squad", "specs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	path := filepath.Join(dir, name+".md")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "spec %s already exists\n", path)
		return 4
	}
	body := fmt.Sprintf(specStub, yamlInlineTitle(title))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 4
	}
	fmt.Fprintln(stdout, path)
	return 0
}

func yamlInlineTitle(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#\"\n\r") || strings.HasPrefix(s, "-") {
		return fmt.Sprintf("%q", s)
	}
	return s
}
