// Package scaffold renders templates and writes scaffolded files for
// `squad init`. Templates live under internal/scaffold/templates/ and are
// embedded via embed.FS.
package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed all:templates
var Templates embed.FS

type Data struct {
	ProjectName     string
	IDPrefixes      []string
	PrimaryLanguage string
	GitRoot         string
	Remote          string
	RepoID          string
	InstallPlugin   bool
}

func Render(tmpl string, data Data) (string, error) {
	t, err := template.New("squad").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute: %w", err)
	}
	return buf.String(), nil
}
