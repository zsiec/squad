package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfig_CreatesYAML(t *testing.T) {
	root := t.TempDir()
	d := Data{ProjectName: "octopus", IDPrefixes: []string{"BUG", "FEAT"}, PrimaryLanguage: "go"}
	if err := WriteConfig(root, d); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".squad", "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "project_name: octopus") {
		t.Fatalf("missing project_name; got:\n%s", got)
	}
}

func TestWriteConfig_IsIdempotent(t *testing.T) {
	root := t.TempDir()
	d := Data{ProjectName: "p", IDPrefixes: []string{"BUG"}, PrimaryLanguage: "go"}
	if err := WriteConfig(root, d); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(root, ".squad", "config.yaml"))
	if err := WriteConfig(root, d); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(root, ".squad", "config.yaml"))
	if string(first) != string(second) {
		t.Fatalf("not idempotent; first:\n%s\nsecond:\n%s", first, second)
	}
}

func TestWriteConfig_DoesNotClobberUserEdits(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".squad")
	_ = os.MkdirAll(dir, 0o755)
	custom := []byte("# user edited this\nproject_name: handcrafted\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfig(root, Data{ProjectName: "p"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if !strings.Contains(string(got), "handcrafted") {
		t.Fatalf("clobbered user edits; got:\n%s", got)
	}
}

func TestWriteStatus_CreatesBoard(t *testing.T) {
	root := t.TempDir()
	if err := WriteStatus(root, Data{ProjectName: "octopus"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".squad", "STATUS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "# octopus — backlog board") {
		t.Fatalf("missing header; got:\n%s", got)
	}
}

func TestWriteExampleItem_CreatesItemFile(t *testing.T) {
	root := t.TempDir()
	if err := WriteExampleItem(root, Data{ProjectName: "octopus"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".squad", "items", "EXAMPLE-001-try-the-loop.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "id: EXAMPLE-001") {
		t.Fatalf("missing frontmatter; got:\n%s", got)
	}
}

func TestWriteExampleItem_SkipsIfPresent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".squad", "items")
	_ = os.MkdirAll(dir, 0o755)
	custom := []byte("user-edited\n")
	_ = os.WriteFile(filepath.Join(dir, "EXAMPLE-001-try-the-loop.md"), custom, 0o644)
	if err := WriteExampleItem(root, Data{ProjectName: "p"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "EXAMPLE-001-try-the-loop.md"))
	if string(got) != "user-edited\n" {
		t.Fatalf("clobbered user edits; got %q", got)
	}
}

func TestWriteAgents_CreatesFile(t *testing.T) {
	root := t.TempDir()
	d := Data{ProjectName: "octopus", IDPrefixes: []string{"BUG", "FEAT"}}
	if err := WriteAgents(root, d); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# octopus — Agent Operating Manual", "## §1 — Resume", "## §13 — When in doubt"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestWriteAgents_SkipsIfPresent(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("user-managed"), 0o644)
	if err := WriteAgents(root, Data{ProjectName: "p"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(got) != "user-managed" {
		t.Fatalf("clobbered AGENTS.md; got %q", got)
	}
}
