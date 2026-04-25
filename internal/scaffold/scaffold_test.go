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

func TestStatusTemplate_RendersWithProject(t *testing.T) {
	raw, err := Templates.ReadFile("templates/status.md.tmpl")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got, err := Render(string(raw), Data{ProjectName: "octopus"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"# octopus — backlog board", "## In Progress", "## Ready", "## Blocked", "## Recent Done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status template missing %q", want)
		}
	}
}

func TestExampleItem_HasFrontmatterAndBody(t *testing.T) {
	raw, err := Templates.ReadFile("templates/items/EXAMPLE-001-try-the-loop.md.tmpl")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got, err := Render(string(raw), Data{ProjectName: "octopus"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"id: EXAMPLE-001",
		"title:",
		"status: open",
		"priority: P3",
		"## Problem",
		"## Acceptance criteria",
		"squad claim EXAMPLE-001",
		"squad done EXAMPLE-001",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("example item missing %q", want)
		}
	}
}

func TestConfigTemplate_RendersAllKnobs(t *testing.T) {
	raw, err := Templates.ReadFile("templates/config.yaml.tmpl")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got, err := Render(string(raw), Data{
		ProjectName:     "octopus",
		IDPrefixes:      []string{"BUG", "FEAT", "TASK", "CHORE"},
		PrimaryLanguage: "go",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"project_name: octopus",
		"id_prefixes:",
		"- BUG",
		"- FEAT",
		"agent:",
		"verification:",
		"hygiene:",
		"stale_claim_minutes:",
		"sweep_on_every_command:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("config template missing %q", want)
		}
	}
}

func TestAgentsTemplate_HasAllSections(t *testing.T) {
	raw, err := Templates.ReadFile("templates/AGENTS.md.tmpl")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got, err := Render(string(raw), Data{ProjectName: "octopus", IDPrefixes: []string{"BUG", "FEAT", "TASK"}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"# octopus — Agent Operating Manual",
		"## §0 — Mental model",
		"## §1 — Resume a session",
		"## §2 — Pick an item",
		"## §3 — Work the item",
		"## §4 — Test before claiming done",
		"## §5 — Code review",
		"## §6 — Commit and close",
		"## §7 — Item file template",
		"## §8 — Anti-patterns",
		"## §9 — When in doubt, ask",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("AGENTS template missing %q", want)
		}
	}
}

func TestAllTemplates_NoSwitchframeVocabulary(t *testing.T) {
	banned := []string{
		"Switchframe", "switchframe", "sf-team", "MXL", "SCTE",
		"multi-region", "make demo", "server/output/", "server/audio/",
		"server/cmdqueue/", "server/clock/", "server/peer/",
		"~/.switchframe-team",
	}
	files := []string{
		"templates/AGENTS.md.tmpl",
		"templates/claude.md.fragment.tmpl",
		"templates/config.yaml.tmpl",
		"templates/status.md.tmpl",
		"templates/items/EXAMPLE-001-try-the-loop.md.tmpl",
	}
	for _, f := range files {
		raw, err := Templates.ReadFile(f)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", f, err)
		}
		for _, b := range banned {
			if strings.Contains(string(raw), b) {
				t.Fatalf("%s leaks Switchframe-ism %q", f, b)
			}
		}
	}
}

func TestClaudeFragment_HasMarkersAndPointers(t *testing.T) {
	raw, err := Templates.ReadFile("templates/claude.md.fragment.tmpl")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got, err := Render(string(raw), Data{ProjectName: "octopus"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{
		"<!-- squad-managed:start -->",
		"<!-- squad-managed:end -->",
		"AGENTS.md",
		".squad/items/",
		"squad register",
		"squad tick",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("claude fragment missing %q", want)
		}
	}
}
