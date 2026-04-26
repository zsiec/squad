package learning

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)

func Parse(path string) (Learning, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Learning{}, err
	}
	m := frontmatterRe.FindSubmatch(data)
	if m == nil {
		return Learning{}, fmt.Errorf("no frontmatter in %s", path)
	}
	var l Learning
	if err := yaml.Unmarshal(m[1], &l); err != nil {
		return Learning{}, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}
	if _, err := ParseKind(string(l.Kind)); err != nil {
		return Learning{}, fmt.Errorf("frontmatter in %s: %w", path, err)
	}
	if _, err := ParseState(string(l.State)); err != nil {
		return Learning{}, fmt.Errorf("frontmatter in %s: %w", path, err)
	}
	if strings.TrimSpace(l.Slug) == "" {
		return Learning{}, fmt.Errorf("frontmatter in %s missing slug", path)
	}
	l.Body = string(data[len(m[0]):])
	l.Path = path
	if err := requireSubtypeHeaders(l); err != nil {
		return Learning{}, fmt.Errorf("frontmatter in %s: %w", path, err)
	}
	return l, nil
}

var subtypeHeaders = map[Kind][]string{
	KindGotcha:  {"## Looks like", "## Is"},
	KindPattern: {"## When", "## Do", "## Why"},
	KindDeadEnd: {"## Tried", "## Doesn't work because"},
}

func requireSubtypeHeaders(l Learning) error {
	for _, h := range subtypeHeaders[l.Kind] {
		if !strings.Contains(l.Body, h) {
			return fmt.Errorf("missing required %q header for kind %s", h, l.Kind)
		}
	}
	return nil
}
