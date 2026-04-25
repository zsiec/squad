package scaffold

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	cases := []struct {
		name    string
		tmpl    string
		data    Data
		wantSub string
		wantErr bool
	}{
		{
			name:    "project name substitutes",
			tmpl:    "project: {{.ProjectName}}\n",
			data:    Data{ProjectName: "octopus"},
			wantSub: "project: octopus",
		},
		{
			name:    "id prefixes substitute",
			tmpl:    "prefixes: {{range .IDPrefixes}}{{.}} {{end}}",
			data:    Data{IDPrefixes: []string{"BUG", "FEAT"}},
			wantSub: "prefixes: BUG FEAT ",
		},
		{
			name:    "primary language substitutes",
			tmpl:    "lang: {{.PrimaryLanguage}}",
			data:    Data{PrimaryLanguage: "go"},
			wantSub: "lang: go",
		},
		{
			name:    "missing field is empty, not error",
			tmpl:    "x: {{.ProjectName}} y: {{.Remote}}",
			data:    Data{ProjectName: "p"},
			wantSub: "x: p y: ",
		},
		{
			name:    "bad template fails",
			tmpl:    "x: {{.NoSuchField}}",
			data:    Data{},
			wantErr: true,
		},
		{
			name:    "syntax error fails",
			tmpl:    "x: {{.ProjectName",
			data:    Data{},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Render(tc.tmpl, tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("want substring %q in %q", tc.wantSub, got)
			}
		})
	}
}

func TestTemplatesEmbed_HasEntries(t *testing.T) {
	entries, err := Templates.ReadDir("templates")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry under templates/")
	}
}
