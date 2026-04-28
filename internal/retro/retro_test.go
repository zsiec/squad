package retro

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/store"
)

// All seed times use this base; tests pick offsets within the
// period [base, base+7d) to land inside the configured Period.
var base = time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC) // Monday week 17

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedItem(t *testing.T, db *sql.DB, id, status, typ string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO items (repo_id, item_id, title, type, priority, area,
		status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
		VALUES ('repo-1', ?, 't', ?, 'P2', 'core', ?, '', 'low', 0, 0, 0, '', ?)`,
		id, typ, status, base.Add(2*time.Hour).Unix())
	if err != nil {
		t.Fatal(err)
	}
}

func seedClaimHistory(t *testing.T, db *sql.DB, itemID string, claimed, released time.Time, outcome string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO claim_history (repo_id, item_id, agent_id,
		claimed_at, released_at, outcome) VALUES ('repo-1', ?, 'agent-a', ?, ?, ?)`,
		itemID, claimed.Unix(), released.Unix(), outcome)
	if err != nil {
		t.Fatal(err)
	}
}

func seedAttest(t *testing.T, db *sql.DB, itemID, kind string, exitCode int, at time.Time) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO attestations (item_id, kind, command, exit_code, output_hash,
		output_path, created_at, agent_id, repo_id)
		VALUES (?, ?, 'cmd', ?, ?, '', ?, 'agent-a', 'repo-1')`,
		itemID, kind, exitCode, itemID+kind, at.Unix())
	if err != nil {
		t.Fatal(err)
	}
}

func periodForBase() Period {
	return Period{
		Year:  2026,
		Week:  17,
		Since: base.Unix(),
		Until: base.Add(7 * 24 * time.Hour).Unix(),
	}
}

// TestParseWeek covers AC#4: --week YYYY-WW resolves to a week-long
// span that aligns with ISO-8601 week boundaries (Mon 00:00 UTC →
// next Mon 00:00 UTC).
func TestParseWeek(t *testing.T) {
	cases := []struct {
		in        string
		wantYear  int
		wantWeek  int
		wantSince time.Time
	}{
		{"2026-W17", 2026, 17, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)},
		{"2026-17", 2026, 17, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseWeek(c.in)
			if err != nil {
				t.Fatalf("ParseWeek(%q): %v", c.in, err)
			}
			if got.Year != c.wantYear || got.Week != c.wantWeek {
				t.Errorf("year=%d week=%d, want %d/%d", got.Year, got.Week, c.wantYear, c.wantWeek)
			}
			if got.Since != c.wantSince.Unix() {
				t.Errorf("Since=%d, want %d (%s)", got.Since, c.wantSince.Unix(), c.wantSince)
			}
			if got.Until != c.wantSince.Add(7*24*time.Hour).Unix() {
				t.Errorf("Until=%d, want Since+7d", got.Until)
			}
		})
	}
}

func TestParseWeek_Invalid(t *testing.T) {
	for _, s := range []string{"", "abc", "2026", "2026-W", "2026-W99", "9999-W01"} {
		if _, err := ParseWeek(s); err == nil {
			t.Errorf("ParseWeek(%q) expected error", s)
		}
	}
}

// TestCurrentWeek covers the default case used by `squad retro` with
// no --week flag: take the ISO-week containing `now`.
func TestCurrentWeek(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 30, 0, 0, time.UTC) // Wed week 17
	p := CurrentWeek(now)
	if p.Year != 2026 || p.Week != 17 {
		t.Errorf("year/week = %d/%d, want 2026/17", p.Year, p.Week)
	}
	wantSince := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC).Unix()
	if p.Since != wantSince {
		t.Errorf("Since=%d, want %d (Mon 00:00)", p.Since, wantSince)
	}
}

// TestGenerate_BelowThreshold covers AC#6: <MinItems closed claims in
// the period yields a one-line insufficient-signal body and ok=false,
// so the cron caller writes a placeholder instead of an empty retro.
func TestGenerate_BelowThreshold(t *testing.T) {
	db := openTestDB(t)
	seedItem(t, db, "BUG-1", "done", "bug")
	seedClaimHistory(t, db, "BUG-1", base.Add(time.Hour), base.Add(2*time.Hour), "done")

	body, ok, err := Generate(context.Background(), db, Opts{
		RepoID: "repo-1", Period: periodForBase(), MinItems: 5,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if ok {
		t.Errorf("ok=true, want false (1 claim < threshold 5)")
	}
	if !strings.Contains(body, "insufficient signal") {
		t.Errorf("body missing 'insufficient signal' marker: %q", body)
	}
}

// TestGenerate_AllThreeSections covers AC#1: the retro has Top failure
// modes, Slowest item type, and Recommendation sections at minimum.
func TestGenerate_AllThreeSections(t *testing.T) {
	db := openTestDB(t)
	for i, st := range []string{"done", "done", "done", "blocked", "blocked", "released"} {
		id := "BUG-" + string(rune('A'+i))
		seedItem(t, db, id, st, "bug")
		seedClaimHistory(t, db, id,
			base.Add(time.Duration(i)*time.Hour),
			base.Add(time.Duration(i)*time.Hour+30*time.Minute),
			st)
	}
	seedAttest(t, db, "BUG-A", "test", 1, base.Add(time.Hour))
	seedAttest(t, db, "BUG-B", "test", 1, base.Add(2*time.Hour))

	body, ok, err := Generate(context.Background(), db, Opts{
		RepoID: "repo-1", Period: periodForBase(), MinItems: 5,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !ok {
		t.Fatalf("ok=false, want true (6 claims >= 5)")
	}
	for _, want := range []string{
		"# Retro 2026-W17",
		"## Top failure modes",
		"## Slowest item type",
		"## Recommendation",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q. body=\n%s", want, body)
		}
	}
}

// TestGenerate_Deterministic covers AC#2: same DB, same body byte-for-byte.
func TestGenerate_Deterministic(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 6; i++ {
		id := "FEAT-" + string(rune('1'+i))
		seedItem(t, db, id, "done", "feat")
		seedClaimHistory(t, db, id,
			base.Add(time.Duration(i)*time.Hour),
			base.Add(time.Duration(i)*time.Hour+45*time.Minute),
			"done")
	}
	a, _, err := Generate(context.Background(), db, Opts{RepoID: "repo-1", Period: periodForBase(), MinItems: 5})
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Generate(context.Background(), db, Opts{RepoID: "repo-1", Period: periodForBase(), MinItems: 5})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("non-deterministic output:\nA=\n%s\nB=\n%s", a, b)
	}
}

// TestGenerate_FailureModesIncludeBlocked: blocked items in the
// period must surface in the failure-modes section. This is the
// concrete "items closed as `blocked`" example from AC#1.
func TestGenerate_FailureModesIncludeBlocked(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 3; i++ {
		id := "BUG-B" + string(rune('1'+i))
		seedItem(t, db, id, "blocked", "bug")
		seedClaimHistory(t, db, id, base.Add(time.Hour), base.Add(2*time.Hour), "released")
	}
	for i := 0; i < 3; i++ {
		id := "BUG-D" + string(rune('1'+i))
		seedItem(t, db, id, "done", "bug")
		seedClaimHistory(t, db, id, base.Add(time.Hour), base.Add(2*time.Hour), "done")
	}
	body, ok, err := Generate(context.Background(), db, Opts{RepoID: "repo-1", Period: periodForBase(), MinItems: 5})
	if err != nil || !ok {
		t.Fatalf("Generate: ok=%v err=%v", ok, err)
	}
	if !strings.Contains(strings.ToLower(body), "blocked") {
		t.Errorf("failure modes missing 'blocked':\n%s", body)
	}
}

// TestGenerate_SlowestTypeIdentified: types with longer median close
// time are surfaced. We seed two types with very different durations.
func TestGenerate_SlowestTypeIdentified(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 3; i++ {
		id := "FEAT-S" + string(rune('1'+i))
		seedItem(t, db, id, "done", "feat")
		seedClaimHistory(t, db, id, base, base.Add(8*time.Hour), "done")
	}
	for i := 0; i < 3; i++ {
		id := "BUG-Q" + string(rune('1'+i))
		seedItem(t, db, id, "done", "bug")
		seedClaimHistory(t, db, id, base, base.Add(30*time.Minute), "done")
	}
	body, ok, err := Generate(context.Background(), db, Opts{RepoID: "repo-1", Period: periodForBase(), MinItems: 5})
	if err != nil || !ok {
		t.Fatalf("Generate: ok=%v err=%v", ok, err)
	}
	slow := body[strings.Index(body, "## Slowest item type"):]
	if !strings.Contains(slow, "feat") {
		t.Errorf("Slowest type should be 'feat' (8h median):\n%s", slow)
	}
}

// TestGenerate_FailingAttestationsCountTowardThreshold: a week with
// zero closes but plenty of failing attestations is real signal (CI
// thrashing) — the threshold check must include that signal so the
// retro doesn't silently swallow it.
func TestGenerate_FailingAttestationsCountTowardThreshold(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 6; i++ {
		id := "BUG-CI" + string(rune('1'+i))
		seedAttest(t, db, id, "test", 1, base.Add(time.Duration(i)*time.Hour))
	}
	body, ok, err := Generate(context.Background(), db, Opts{RepoID: "repo-1", Period: periodForBase(), MinItems: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Errorf("ok=false; want true (6 failing attestations >= 5)")
	}
	if !strings.Contains(body, "attestation kind `test` failures") {
		t.Errorf("body should call out failing attestation kind:\n%s", body)
	}
}

// TestGenerate_OutsidePeriodIgnored: rows outside [Since, Until) must
// not contribute to the digest. Otherwise the file isn't really a
// "weekly" retro.
func TestGenerate_OutsidePeriodIgnored(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 6; i++ {
		id := "OLD-" + string(rune('1'+i))
		seedItem(t, db, id, "done", "feat")
		// Released two weeks BEFORE the period.
		old := base.Add(-15 * 24 * time.Hour)
		seedClaimHistory(t, db, id, old, old.Add(time.Hour), "done")
	}
	_, ok, err := Generate(context.Background(), db, Opts{RepoID: "repo-1", Period: periodForBase(), MinItems: 5})
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Errorf("ok=true with all rows outside period; want insufficient-signal")
	}
}
