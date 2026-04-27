package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeriveQuickSlug(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain words", "interface in claims store breaks", "interface-in-claims-store-breaks"},
		{"mixed case", "SQLITE_BUSY across fork", "sqlite-busy-across-fork"},
		{"punctuation collapses", "go 1.25!! pure-go: no CGO???", "go-1-25-pure-go-no-cgo"},
		{"leading non-alpha stripped", "12 angry men of go", "angry-men-of-go"},
		{"trailing dashes trimmed", "trailing punctuation!!!", "trailing-punctuation"},
		{"runs of dashes collapsed", "a -- b -- c", "a-b-c"},
		{"max length 60", "a very long surprise message that goes on and on and on past sixty characters", "a-very-long-surprise-message-that-goes-on-and-on-and-on-past"},
		{"unicode dropped not transliterated", "café broke on Linux", "caf-broke-on-linux"},
		{"emoji becomes dash", "ship it 🚀 fast", "ship-it-fast"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveQuickSlug(tc.in)
			if got != tc.want {
				t.Fatalf("deriveQuickSlug(%q) = %q; want %q", tc.in, got, tc.want)
			}
			if len(got) > 60 {
				t.Fatalf("len(%q) = %d > 60", got, len(got))
			}
			if got != "" && !validSlug(got) {
				t.Fatalf("derived slug %q must satisfy validSlug", got)
			}
		})
	}
}

func TestDeriveQuickSlug_TooShort(t *testing.T) {
	cases := []string{"", "!!", "ab", "1", "?-?"}
	for _, in := range cases {
		got := deriveQuickSlug(in)
		if got != "" {
			t.Errorf("deriveQuickSlug(%q) = %q; want empty (too short to be a usable slug)", in, got)
		}
	}
}

func TestInferQuickArea_FromMostRecentDoneItem(t *testing.T) {
	repo := t.TempDir()
	doneDir := filepath.Join(repo, ".squad", "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	older := filepath.Join(doneDir, "BUG-101-old.md")
	newer := filepath.Join(doneDir, "TASK-202-new.md")
	mkItem(t, older, "BUG-101", "cli")
	mkItem(t, newer, "TASK-202", "store")
	// make `newer` deterministically more recent
	past := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}

	got := inferQuickArea(repo)
	if got != "store" {
		t.Fatalf("inferQuickArea = %q; want %q", got, "store")
	}
}

func TestInferQuickArea_FallbackGeneral(t *testing.T) {
	repo := t.TempDir()
	if got := inferQuickArea(repo); got != "general" {
		t.Fatalf("inferQuickArea on empty repo = %q; want %q", got, "general")
	}

	// empty done dir, still "general"
	if err := os.MkdirAll(filepath.Join(repo, ".squad", "done"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := inferQuickArea(repo); got != "general" {
		t.Fatalf("inferQuickArea on empty done dir = %q; want %q", got, "general")
	}
}

func TestInferQuickArea_MissingAreaFallsBack(t *testing.T) {
	repo := t.TempDir()
	doneDir := filepath.Join(repo, ".squad", "done")
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// item with no area frontmatter
	mkItem(t, filepath.Join(doneDir, "BUG-303-nofield.md"), "BUG-303", "")
	if got := inferQuickArea(repo); got != "general" {
		t.Fatalf("inferQuickArea = %q; want fallback %q", got, "general")
	}
}

func TestLearningQuick_HappyPath(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "quick", "interface{} in claims store breaks Go 1.25"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr=%s", err, stderr.String())
	}

	wantSlug := "interface-in-claims-store-breaks-go-1-25"
	wantPath := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed", wantSlug+".md")
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected stub at %s: %v", wantPath, err)
	}
	for _, w := range []string{
		"kind: gotcha",
		"slug: " + wantSlug,
		"title: interface{} in claims store breaks Go 1.25",
		"area: general",
		"state: proposed",
		"## Looks like",
		"> captured via squad learning quick",
	} {
		if !strings.Contains(string(body), w) {
			t.Errorf("stub missing %q\n---\n%s", w, body)
		}
	}
	if !strings.Contains(stdout.String(), wantPath) {
		t.Errorf("stdout = %q, want path", stdout.String())
	}
	if !strings.Contains(stderr.String(), "edit the stub") {
		t.Errorf("stderr should contain follow-up nudge, got %q", stderr.String())
	}
}

func TestLearningQuick_KindOverride(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	root := newRootCmd()
	root.SetArgs([]string{"learning", "quick", "use channel of done to fan out workers", "--kind", "pattern"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	wantPath := filepath.Join(repo, ".squad", "learnings", "patterns", "proposed", "use-channel-of-done-to-fan-out-workers.md")
	body, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected stub at %s: %v", wantPath, err)
	}
	for _, w := range []string{"kind: pattern", "## When", "## Do", "## Why"} {
		if !strings.Contains(string(body), w) {
			t.Errorf("stub missing %q for pattern kind\n---\n%s", w, body)
		}
	}
}

func TestLearningQuick_SlugCollisionWalksSuffix(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	// pre-seed an existing proposal that owns the natural slug
	mkLearning(t, repo, "gotchas/proposed/database-locked-error-on-fork.md",
		"gotcha", "database-locked-error-on-fork", "store", "proposed")

	root := newRootCmd()
	root.SetArgs([]string{"learning", "quick", "database locked error on fork"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	suffixed := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed",
		"database-locked-error-on-fork-2.md")
	if _, err := os.Stat(suffixed); err != nil {
		t.Fatalf("expected -2 suffix file at %s: %v", suffixed, err)
	}

	// Original is unchanged.
	original := filepath.Join(repo, ".squad", "learnings", "gotchas", "proposed",
		"database-locked-error-on-fork.md")
	body, err := os.ReadFile(original)
	if err != nil {
		t.Fatalf("original deleted: %v", err)
	}
	if !strings.Contains(string(body), "## Looks like\n\nx") {
		t.Errorf("original body lost — quick should not touch existing files")
	}
}

func TestLearningQuick_TooShortOneLiner(t *testing.T) {
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	root := newRootCmd()
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "quick", "ab"})
	err := root.Execute()
	if err == nil {
		t.Fatalf("want error for too-short one-liner, got none")
	}
	combined := err.Error() + stderr.String()
	if !strings.Contains(combined, "shorter than 3") && !strings.Contains(combined, "more specific") {
		t.Errorf("expected helpful too-short message, got %q / %s", err, stderr.String())
	}
}

func TestLearningQuick_SilencedEnvNoNudge(t *testing.T) {
	t.Setenv("SQUAD_NO_CADENCE_NUDGES", "1")
	repo := setupSquadRepo(t)
	t.Chdir(repo)

	var stdout, stderr bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"learning", "quick", "silent surprise about handlers"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(stderr.String(), "edit the stub") {
		t.Errorf("stderr should be silent under SQUAD_NO_CADENCE_NUDGES=1, got %q", stderr.String())
	}
}

func mkItem(t *testing.T, path, id, area string) {
	t.Helper()
	areaLine := ""
	if area != "" {
		areaLine = "area: " + area + "\n"
	}
	body := "---\n" +
		"id: " + id + "\ntitle: t\ntype: bug\npriority: P3\nstatus: done\n" +
		areaLine +
		"---\n\nbody\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
