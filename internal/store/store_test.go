package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpen_AppliesSchemaIdempotently(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "global.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"repos", "agents", "claims", "claim_history", "messages", "touches", "reads", "progress"} {
		var name string
		if err := db.QueryRowContext(context.Background(),
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	db2.Close()
}

func TestOpen_RepoIdColumnPresent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"agents", "claims", "claim_history", "messages", "touches"} {
		rows, err := db.QueryContext(context.Background(),
			"SELECT name FROM pragma_table_info(?)", table)
		if err != nil {
			t.Fatalf("pragma_table_info(%s): %v", table, err)
		}
		found := false
		for rows.Next() {
			var col string
			if err := rows.Scan(&col); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if col == "repo_id" {
				found = true
			}
		}
		rows.Close()
		if !found {
			t.Fatalf("table %s missing repo_id column", table)
		}
	}
}

func TestBeginImmediate_CommitsWrite(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := BeginImmediate(context.Background(), db)
	if err != nil {
		t.Fatalf("BeginImmediate: %v", err)
	}
	if _, err := tx.ExecContext(context.Background(),
		`INSERT INTO repos (id, root_path, remote_url, name, created_at) VALUES (?, ?, ?, ?, ?)`,
		"abc123", "/tmp/x", "git@github.com:foo/bar.git", "bar", 1700000000); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var got string
	if err := db.QueryRow(`SELECT id FROM repos WHERE id=?`, "abc123").Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != "abc123" {
		t.Fatalf("got %q", got)
	}
}

func TestSchema_NotifyEndpointsTableExists(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		INSERT INTO notify_endpoints (instance, repo_id, kind, port, started_at)
		VALUES ('inst-1', 'repo-x', 'listen', 51234, 1700000000)
	`); err != nil {
		t.Fatalf("insert into notify_endpoints: %v", err)
	}

	var port int
	if err := db.QueryRow(
		`SELECT port FROM notify_endpoints WHERE instance='inst-1'`,
	).Scan(&port); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if port != 51234 {
		t.Fatalf("port=%d", port)
	}
}

func TestSchema_NotifyEndpoints_PrimaryKeyOnInstanceKind(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`INSERT INTO notify_endpoints (instance, repo_id, kind, port, started_at) VALUES ('inst-1','repo-x','listen',9001,1)`); err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	_, err = db.Exec(`INSERT INTO notify_endpoints (instance, repo_id, kind, port, started_at) VALUES ('inst-1','repo-x','listen',9002,2)`)
	if err == nil {
		t.Fatalf("expected unique-constraint failure on (instance, kind), got nil")
	}
}

func TestSchema_ItemsHasR3Columns(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name FROM pragma_table_info('items')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		cols[n] = true
	}
	for _, c := range []string{"epic_id", "parallel", "conflicts_with"} {
		if !cols[c] {
			t.Errorf("items missing column %q", c)
		}
	}
}

func TestSchema_AttestationsTable(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(filepath.Join(dir, "global.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	const insert = `
		INSERT INTO attestations (item_id, kind, command, exit_code, output_hash, output_path, created_at, agent_id, repo_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if _, err := db.Exec(insert,
		"FEAT-001", "test", "go test ./...", 0,
		"a1b2c3", ".squad/attestations/a1b2c3.txt",
		1700000000, "agent-x", "repo-y"); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var got struct {
		ItemID, Kind, Command, OutputHash, OutputPath, AgentID, RepoID string
		ExitCode                                                       int
		CreatedAt                                                      int64
	}
	row := db.QueryRow(`SELECT item_id, kind, command, exit_code, output_hash, output_path, created_at, agent_id, repo_id FROM attestations WHERE item_id = ?`, "FEAT-001")
	if err := row.Scan(&got.ItemID, &got.Kind, &got.Command, &got.ExitCode, &got.OutputHash, &got.OutputPath, &got.CreatedAt, &got.AgentID, &got.RepoID); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if got.ItemID != "FEAT-001" || got.Kind != "test" || got.ExitCode != 0 || got.OutputHash != "a1b2c3" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	if _, err := db.Exec(insert,
		"FEAT-001", "test", "go test ./...", 0,
		"a1b2c3", ".squad/attestations/a1b2c3.txt",
		1700000001, "agent-x", "repo-y"); err == nil {
		t.Fatalf("expected unique-violation on (item_id, output_hash)")
	}
}

func TestSchema_SpecsAndEpicsTablesExist(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"specs", "epics"} {
		var n int
		_ = db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`,
			table).Scan(&n)
		if n != 1 {
			t.Errorf("table %q missing (count=%d)", table, n)
		}
	}
}
