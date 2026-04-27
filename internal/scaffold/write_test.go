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

func TestWriteCapturedExampleItem_CreatesItemFile(t *testing.T) {
	root := t.TempDir()
	if err := WriteCapturedExampleItem(root, Data{ProjectName: "octopus"}); err != nil {
		t.Fatalf("WriteCapturedExampleItem: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".squad", "items", "IDEA-001-something-to-think-about.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "id: IDEA-001") {
		t.Fatalf("missing frontmatter id; got:\n%s", got)
	}
	if !strings.Contains(string(got), "status: captured") {
		t.Fatalf("missing status: captured; got:\n%s", got)
	}
}

func TestWriteCapturedExampleItem_SkipsIfPresent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".squad", "items")
	_ = os.MkdirAll(dir, 0o755)
	custom := []byte("user-edited\n")
	_ = os.WriteFile(filepath.Join(dir, "IDEA-001-something-to-think-about.md"), custom, 0o644)
	if err := WriteCapturedExampleItem(root, Data{ProjectName: "p"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "IDEA-001-something-to-think-about.md"))
	if string(got) != "user-edited\n" {
		t.Fatalf("clobbered user edits; got %q", got)
	}
}

func TestWriteConfig_DefaultsWorktreePerClaimTrue(t *testing.T) {
	root := t.TempDir()
	d := Data{ProjectName: "p", IDPrefixes: []string{"BUG"}, PrimaryLanguage: "go"}
	if err := WriteConfig(root, d); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, ".squad", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "default_worktree_per_claim: true") {
		t.Fatalf("config.yaml does not opt into worktree-per-claim by default; got:\n%s", got)
	}
}

func TestWriteAgents_DocumentsWorktreeDefaultAndOptOut(t *testing.T) {
	root := t.TempDir()
	if err := WriteAgents(root, Data{ProjectName: "p", IDPrefixes: []string{"BUG"}}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"default_worktree_per_claim: true", "default_worktree_per_claim: false"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("AGENTS.md missing worktree default/opt-out mention %q; got:\n%s", want, got)
		}
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
	for _, want := range []string{"# octopus — Agent Operating Manual", "## §1 — Resume a session", "## §14 — When in doubt, ask"} {
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

// AGENTS.md absorbed the former docs/agents-deep.md content — the multi-agent,
// handoff, chat-cadence, time-boxing, and anchor-checkpoint sections must
// surface in AGENTS.md itself, not a separate deep file.
func TestWriteAgents_CarriesFormerlyDeepSections(t *testing.T) {
	repo := t.TempDir()
	d := Data{ProjectName: "Test", IDPrefixes: []string{"BUG", "FEAT"}}
	if err := WriteAgents(repo, d); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(repo, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Multi-agent dispatch",
		"Handoff",
		"Chat cadence",
		"Time-boxing",
		"Anchor checkpoints",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("AGENTS.md missing section %q", want)
		}
	}
}
