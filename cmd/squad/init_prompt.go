package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/zsiec/squad/internal/scaffold"
)

var defaultPrefixes = []string{"BUG", "FEAT", "TASK", "CHORE"}

type answers struct {
	ProjectName   string
	IDPrefixes    []string
	InstallPlugin bool
}

func promptAnswers(info scaffold.RepoInfo, yes bool, in io.Reader, out io.Writer) (answers, error) {
	if yes {
		return answers{
			ProjectName:   info.ProjectBasename,
			IDPrefixes:    append([]string{}, defaultPrefixes...),
			InstallPlugin: true,
		}, nil
	}
	r := bufio.NewReader(in)

	fmt.Fprintf(out, "Project name [%s]: ", info.ProjectBasename)
	name, err := readLine(r)
	if err != nil {
		return answers{}, err
	}
	if name == "" {
		name = info.ProjectBasename
	}

	fmt.Fprintf(out, "ID prefixes [%s]: ", strings.Join(defaultPrefixes, ","))
	raw, err := readLine(r)
	if err != nil {
		return answers{}, err
	}
	prefixes := defaultPrefixes
	if raw != "" {
		prefixes = parsePrefixes(raw)
	}

	fmt.Fprint(out, "Install Claude Code plugin? [Y/n]: ")
	plug, err := readLine(r)
	if err != nil {
		return answers{}, err
	}
	install := !strings.EqualFold(plug, "n") && !strings.EqualFold(plug, "no")

	return answers{ProjectName: name, IDPrefixes: prefixes, InstallPlugin: install}, nil
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func parsePrefixes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
