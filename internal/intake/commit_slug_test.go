package intake

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckSlugAvailable_NeitherExists(t *testing.T) {
	db := newTestDB(t)
	squadDir := t.TempDir()

	if err := CheckSlugAvailable(context.Background(), db, "repo-a", squadDir, "spec", "observability"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := CheckSlugAvailable(context.Background(), db, "repo-a", squadDir, "epic", "tracing"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestCheckSlugAvailable_DBRowConflict_Spec(t *testing.T) {
	db := newTestDB(t)
	squadDir := t.TempDir()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO specs (repo_id, name, title, path, updated_at) VALUES (?, ?, ?, ?, 0)`,
		"repo-a", "observability", "Observability v2", "/tmp/specs/observability.md")
	if err != nil {
		t.Fatalf("seed spec row: %v", err)
	}

	err = CheckSlugAvailable(context.Background(), db, "repo-a", squadDir, "spec", "observability")
	var conflict *IntakeSlugConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("got %v; want *IntakeSlugConflict", err)
	}
	if conflict.Kind != "spec" || conflict.Slug != "observability" {
		t.Errorf("conflict = %+v; want kind=spec slug=observability", conflict)
	}
}

func TestCheckSlugAvailable_DBRowConflict_Epic(t *testing.T) {
	db := newTestDB(t)
	squadDir := t.TempDir()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO epics (repo_id, name, spec, path, updated_at) VALUES (?, ?, ?, ?, 0)`,
		"repo-a", "tracing", "observability", "/tmp/epics/tracing.md")
	if err != nil {
		t.Fatalf("seed epic row: %v", err)
	}

	err = CheckSlugAvailable(context.Background(), db, "repo-a", squadDir, "epic", "tracing")
	var conflict *IntakeSlugConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("got %v; want *IntakeSlugConflict", err)
	}
}

func TestCheckSlugAvailable_FileConflict(t *testing.T) {
	db := newTestDB(t)
	squadDir := t.TempDir()

	specsDir := filepath.Join(squadDir, "specs")
	if err := os.MkdirAll(specsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specsDir, "observability.md"), []byte("---\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	err := CheckSlugAvailable(context.Background(), db, "repo-a", squadDir, "spec", "observability")
	var conflict *IntakeSlugConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("got %v; want *IntakeSlugConflict", err)
	}
	if conflict.Kind != "spec" {
		t.Errorf("kind = %q; want spec", conflict.Kind)
	}
}

func TestCheckSlugAvailable_KindIsolated(t *testing.T) {
	db := newTestDB(t)
	squadDir := t.TempDir()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO specs (repo_id, name, title, path, updated_at) VALUES (?, ?, ?, ?, 0)`,
		"repo-a", "shared-name", "Spec Title", "/tmp/specs/shared-name.md")
	if err != nil {
		t.Fatalf("seed spec row: %v", err)
	}

	if err := CheckSlugAvailable(context.Background(), db, "repo-a", squadDir, "epic", "shared-name"); err != nil {
		t.Errorf("epic kind should be available even though spec slug is taken: %v", err)
	}

	epicsDir := filepath.Join(squadDir, "epics")
	if err := os.MkdirAll(epicsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(epicsDir, "shared-name.md"), []byte("---\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := CheckSlugAvailable(context.Background(), db, "repo-a", squadDir, "epic", "shared-name"); err == nil {
		t.Errorf("epic slug should now be taken via the file: got nil")
	}
}

func TestCheckSlugAvailable_DifferentRepoIsolated(t *testing.T) {
	db := newTestDB(t)
	squadDir := t.TempDir()

	_, err := db.ExecContext(context.Background(),
		`INSERT INTO specs (repo_id, name, title, path, updated_at) VALUES (?, ?, ?, ?, 0)`,
		"repo-a", "observability", "Spec", "/tmp/specs/observability.md")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := CheckSlugAvailable(context.Background(), db, "repo-b", squadDir, "spec", "observability"); err != nil {
		t.Errorf("different repo should not collide: %v", err)
	}
}

func TestCheckSlugAvailable_RejectsUnknownKind(t *testing.T) {
	db := newTestDB(t)
	err := CheckSlugAvailable(context.Background(), db, "repo-a", t.TempDir(), "item", "anything")
	if err == nil {
		t.Fatalf("expected error for unknown kind")
	}
}

func TestIntakeSlugConflict_ErrorFormat(t *testing.T) {
	got := (&IntakeSlugConflict{Kind: "spec", Slug: "observability"}).Error()
	want := "intake slug conflict: spec slug \"observability\" already exists"
	if got != want {
		t.Errorf("Error() = %q\nwant %q", got, want)
	}
}
