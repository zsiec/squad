package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCLAUDE(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestMergeCLAUDE_NoFile_CreatesIt(t *testing.T) {
	root := t.TempDir()
	if err := MergeCLAUDE(root, Data{ProjectName: "octopus"}, ChoiceTop); err != nil {
		t.Fatalf("MergeCLAUDE: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "<!-- squad-managed:start -->") {
		t.Fatalf("missing start marker")
	}
}

func TestMergeCLAUDE_ExistingMarkers_ReplacesBlockOnly(t *testing.T) {
	root := t.TempDir()
	body := "# preamble user owns\n\n" +
		"<!-- squad-managed:start -->\nstale stuff to be replaced\n<!-- squad-managed:end -->\n\n" +
		"# postscript user owns\n"
	writeCLAUDE(t, root, body)

	if err := MergeCLAUDE(root, Data{ProjectName: "octopus"}, ChoiceTop); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	s := string(got)
	if !strings.Contains(s, "preamble user owns") || !strings.Contains(s, "postscript user owns") {
		t.Fatalf("user content destroyed:\n%s", s)
	}
	if strings.Contains(s, "stale stuff") {
		t.Fatalf("stale block not replaced:\n%s", s)
	}
	if !strings.Contains(s, "octopus (managed by squad)") {
		t.Fatalf("rendered fragment missing")
	}
}

func TestMergeCLAUDE_NoMarkers_AppendBottom(t *testing.T) {
	root := t.TempDir()
	writeCLAUDE(t, root, "# my project\n\nIntro paragraph.\n")
	if err := MergeCLAUDE(root, Data{ProjectName: "octopus"}, ChoiceBottom); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	s := string(got)
	if !strings.HasPrefix(s, "# my project") {
		t.Fatalf("user content moved")
	}
	if !strings.Contains(s, "<!-- squad-managed:start -->") {
		t.Fatal("missing block")
	}
}

func TestMergeCLAUDE_NoMarkers_PrependTop(t *testing.T) {
	root := t.TempDir()
	writeCLAUDE(t, root, "# my project\n\nIntro paragraph.\n")
	if err := MergeCLAUDE(root, Data{ProjectName: "octopus"}, ChoiceTop); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	s := string(got)
	if !strings.HasPrefix(s, "<!-- squad-managed:start -->") {
		t.Fatalf("block should be at top")
	}
	if !strings.Contains(s, "Intro paragraph.") {
		t.Fatal("user content lost")
	}
}

func TestMergeCLAUDE_NoMarkers_AbortReturnsError(t *testing.T) {
	root := t.TempDir()
	original := "# my project\n\nKeep me untouched.\n"
	writeCLAUDE(t, root, original)
	err := MergeCLAUDE(root, Data{ProjectName: "octopus"}, ChoiceAbort)
	if !errors.Is(err, ErrMergeAborted) {
		t.Fatalf("want ErrMergeAborted, got %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(got) != original {
		t.Fatalf("file modified on abort")
	}
}

func TestMergeCLAUDE_Idempotent(t *testing.T) {
	root := t.TempDir()
	writeCLAUDE(t, root, "preexisting\n")
	if err := MergeCLAUDE(root, Data{ProjectName: "p"}, ChoiceBottom); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err := MergeCLAUDE(root, Data{ProjectName: "p"}, ChoiceBottom); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if string(first) != string(second) {
		t.Fatalf("not idempotent")
	}
}
