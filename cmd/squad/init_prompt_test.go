package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/scaffold"
)

func TestPrompt_YesUsesDefaults(t *testing.T) {
	info := scaffold.RepoInfo{ProjectBasename: "octopus", PrimaryLanguage: "go"}
	got, err := promptAnswers(info, true, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("promptAnswers: %v", err)
	}
	if got.ProjectName != "octopus" {
		t.Fatalf("project name: want octopus, got %q", got.ProjectName)
	}
	want := []string{"BUG", "FEAT", "TASK", "CHORE"}
	if len(got.IDPrefixes) != len(want) {
		t.Fatalf("id prefixes: want %v, got %v", want, got.IDPrefixes)
	}
	if !got.InstallPlugin {
		t.Fatalf("install plugin should default to true under --yes")
	}
}

func TestPrompt_InteractiveAcceptsAllDefaults(t *testing.T) {
	info := scaffold.RepoInfo{ProjectBasename: "octopus", PrimaryLanguage: "go"}
	in := strings.NewReader("\n\n\n")
	out := &bytes.Buffer{}
	got, err := promptAnswers(info, false, in, out)
	if err != nil {
		t.Fatalf("promptAnswers: %v", err)
	}
	if got.ProjectName != "octopus" {
		t.Fatalf("default project name not used: %q", got.ProjectName)
	}
}

func TestPrompt_InteractiveOverridesProjectName(t *testing.T) {
	info := scaffold.RepoInfo{ProjectBasename: "octopus", PrimaryLanguage: "go"}
	in := strings.NewReader("squidward\n\n\n")
	got, _ := promptAnswers(info, false, in, &bytes.Buffer{})
	if got.ProjectName != "squidward" {
		t.Fatalf("override failed: %q", got.ProjectName)
	}
}

func TestPrompt_InteractiveOverridesPrefixes(t *testing.T) {
	info := scaffold.RepoInfo{ProjectBasename: "p"}
	in := strings.NewReader("\nINFRA, SEC, BUG\n\n")
	got, _ := promptAnswers(info, false, in, &bytes.Buffer{})
	want := []string{"INFRA", "SEC", "BUG"}
	if !equalSlices(got.IDPrefixes, want) {
		t.Fatalf("prefixes: want %v got %v", want, got.IDPrefixes)
	}
}

func TestPrompt_InteractiveDeclinesPlugin(t *testing.T) {
	info := scaffold.RepoInfo{ProjectBasename: "p"}
	in := strings.NewReader("\n\nn\n")
	got, _ := promptAnswers(info, false, in, &bytes.Buffer{})
	if got.InstallPlugin {
		t.Fatal("expected plugin install declined")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
