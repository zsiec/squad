package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAGENTSTemplate_MentionsLearningsTier(t *testing.T) {
	root := t.TempDir()
	d := Data{
		ProjectName: "demo",
		IDPrefixes:  []string{"FEAT", "BUG"},
	}
	if err := WriteAgents(root, d); err != nil {
		t.Fatalf("WriteAgents: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	for _, want := range []string{
		".squad/learnings/",
		"squad learning propose",
		"squad learning approve",
		"squad learning agents-md-suggest",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("AGENTS.md missing mention of %q", want)
		}
	}
}
