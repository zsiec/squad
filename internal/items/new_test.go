package items

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestNew_WritesStubFileAndReturnsPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	path, err := New(dir, "BUG", "Plug a leak in the pump")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "BUG-001-plug-a-leak-in-the-pump.md") {
		t.Fatalf("path=%s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		"id: BUG-001",
		"title: Plug a leak in the pump",
		"type: bug",
		"## Problem",
		"## Acceptance criteria",
		"## Resolution",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
}

// QA round 5 G-1 reproducer: a previous `squad new` whose title corrupted
// the YAML frontmatter wrote a file that Parse rejects. Without
// filename-based ID reservation, the next call would re-issue the same
// numeric and produce two files with the same PREFIX-NN slug.
func TestNew_DoesNotReuseIDFromBrokenFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a file whose frontmatter is unparseable but whose filename
	// clearly carries FEAT-001. (Mimics what happens when title contains a
	// newline or stray colon and the stub renders into invalid YAML.)
	if err := os.WriteFile(filepath.Join(dir, "items", "FEAT-001-broken.md"),
		[]byte("---\nid: FEAT-001\ntitle: oops:\n  nested: bad\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := New(dir, "FEAT", "second")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(path, "FEAT-001-") {
		t.Fatalf("ID re-used despite broken FEAT-001 file already on disk: %s", path)
	}
	if !strings.Contains(path, "FEAT-002-") {
		t.Fatalf("expected FEAT-002, got %s", path)
	}
}

// Titles with colons, quotes, or leading dashes used to render unquoted into
// the stub template, producing frontmatter that yaml.Unmarshal rejected.
// QA round 5 G-1 traced ID-collisions to that pattern. New() should now write
// a file Parse() can round-trip.
func TestNew_HostileTitlesProduceParseableFiles(t *testing.T) {
	cases := []string{
		"normal title",
		"contains: a colon",
		`has "double quotes"`,
		"-leads with a dash",
		"trailing colon:",
		"ends with backslash\\",
		"with #hashtag and -dash",
	}
	for _, title := range cases {
		t.Run(title, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
				t.Fatal(err)
			}
			path, err := New(dir, "FEAT", title)
			if err != nil {
				t.Fatal(err)
			}
			it, err := Parse(path)
			if err != nil {
				t.Fatalf("Parse rejected file written for title %q: %v", title, err)
			}
			if it.Title != title {
				t.Fatalf("title round-trip failed: want %q got %q", title, it.Title)
			}
		})
	}
}

// QA r6-G + H#7: two concurrent `squad new` calls used to TOCTOU on the
// flat NextID lookup and produce two files with the same PREFIX-NN. With
// the items-dir flock, every parallel writer gets a distinct id.
func TestNew_NoDuplicateIDsUnderConcurrency(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	const n = 16
	type result struct {
		path string
		err  error
	}
	results := make(chan result, n)
	var start sync.WaitGroup
	start.Add(1)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			start.Wait()
			p, err := New(dir, "FEAT", "concurrent")
			results <- result{p, err}
			_ = i
		}()
	}
	start.Done()
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("New error: %v", r.err)
		}
		base := filepath.Base(r.path)
		// strip the slug suffix; we only care about PREFIX-NN.
		idPart := base
		if dash := strings.IndexFunc(base, func(r rune) bool { return r == '-' }); dash >= 0 {
			if dash2 := strings.IndexFunc(base[dash+1:], func(r rune) bool { return r == '-' }); dash2 >= 0 {
				idPart = base[:dash+1+dash2]
			}
		}
		if seen[idPart] {
			t.Fatalf("duplicate id %s under concurrency", idPart)
		}
		seen[idPart] = true
	}
}

func TestNew_IncrementsAcrossActiveAndDone(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"items", "done"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "items", "BUG-005-x.md"),
		[]byte("---\nid: BUG-005\ntitle: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "done", "BUG-007-y.md"),
		[]byte("---\nid: BUG-007\ntitle: y\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := New(dir, "BUG", "another")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "BUG-008-another.md") {
		t.Fatalf("path=%s want BUG-008-...", path)
	}
}
