package learning

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLearning(t *testing.T, repoRoot string, k Kind, s State, slug string) {
	t.Helper()
	dir := DirFor(repoRoot, k, s)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	body := strings.Replace(validGotcha,
		"slug: sqlite-busy-on-fork",
		"slug: "+slug, 1)
	body = strings.Replace(body, "state: proposed", "state: "+string(s), 1)
	writeFile(t, filepath.Join(dir, slug+".md"), body)
}

func TestResolveSingle_TypedErrors(t *testing.T) {
	repoRoot := t.TempDir()

	if _, err := ResolveSingle(repoRoot, "missing-slug"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing slug: want errors.Is(err, ErrNotFound), got %v", err)
	}

	writeLearning(t, repoRoot, KindGotcha, StateProposed, "dup-slug")
	writeLearning(t, repoRoot, KindGotcha, StateApproved, "dup-slug")
	if _, err := ResolveSingle(repoRoot, "dup-slug"); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("ambiguous slug: want errors.Is(err, ErrAmbiguous), got %v", err)
	}
}
