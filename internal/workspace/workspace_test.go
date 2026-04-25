package workspace

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestStatus_AggregatesAcrossRepos(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1111111111111", "git@github.com:acme/widgets.git", "/r/widgets", 2*time.Minute)
	f.addRepo(t, "bbb2222222222222", "git@github.com:acme/sprockets.git", "/r/sprockets", 1*time.Hour)
	f.addRepo(t, "ccc3333333333333", "git@github.com:acme/cogs.git", "/r/cogs", 24*time.Hour)
	f.addClaim(t, "aaa1111111111111", "agent-a", "FEAT-1", "wire it up")
	f.addClaim(t, "aaa1111111111111", "agent-a", "BUG-2", "")
	f.addClaim(t, "bbb2222222222222", "agent-b", "FEAT-9", "")

	rows, err := New(f.Store()).Status(context.Background(), Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d: %+v", len(rows), rows)
	}
	// Most active first: aaa1 (just claimed), bbb2 (just claimed), ccc3 (no claims, 24h ago)
	if rows[0].RepoID != "aaa1111111111111" || rows[0].InProgress != 2 {
		t.Fatalf("rows[0] aaa1: %+v", rows[0])
	}
	if rows[1].InProgress != 1 || rows[2].InProgress != 0 {
		t.Fatalf("aggregation wrong: %+v", rows)
	}
}

func TestNext_PrioritySortAcrossRepos(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addRepo(t, "bbb2", "git@github.com:a/y.git", "/y", time.Minute)
	items := []ItemRef{
		{RepoID: "aaa1", ID: "FEAT-1", Priority: "P1", Title: "lower"},
		{RepoID: "bbb2", ID: "BUG-9", Priority: "P0", Title: "stop the bleed"},
		{RepoID: "aaa1", ID: "FEAT-2", Priority: "P2", Title: "later"},
	}
	out, err := newWithItems(f.Store(), items).Next(context.Background(), Filter{}, NextOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].ID != "BUG-9" || out[1].ID != "FEAT-1" {
		t.Fatalf("want [BUG-9, FEAT-1], got %v", ids(out))
	}
}

func TestNext_LimitTruncates(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	out, _ := newWithItems(f.Store(), []ItemRef{
		{RepoID: "aaa1", ID: "FEAT-1", Priority: "P0"},
		{RepoID: "aaa1", ID: "FEAT-2", Priority: "P0"},
		{RepoID: "aaa1", ID: "FEAT-3", Priority: "P0"},
	}).Next(context.Background(), Filter{}, NextOptions{Limit: 2})
	if len(out) != 2 {
		t.Fatalf("limit not honored: got %d", len(out))
	}
}

func TestFilter_CurrentOnly(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addRepo(t, "bbb2", "git@github.com:a/y.git", "/y", time.Minute)
	rows, err := New(f.Store()).Status(context.Background(),
		Filter{Mode: ScopeCurrent, CurrentRepoID: "aaa1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].RepoID != "aaa1" {
		t.Fatalf("ScopeCurrent wrong: %+v", rows)
	}
}

func TestFilter_OtherExcludesCurrent(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addRepo(t, "bbb2", "git@github.com:a/y.git", "/y", time.Minute)
	rows, _ := New(f.Store()).Status(context.Background(),
		Filter{Mode: ScopeOther, CurrentRepoID: "aaa1"})
	if len(rows) != 1 || rows[0].RepoID != "bbb2" {
		t.Fatalf("ScopeOther wrong: %+v", rows)
	}
}

func TestForget_RefusesWhenActiveClaimsExist(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addClaim(t, "aaa1", "agent-a", "FEAT-1", "")
	err := New(f.Store()).Forget(context.Background(), "aaa1", false)
	if err == nil || !strings.Contains(err.Error(), "active claims") {
		t.Fatalf("want 'active claims' refusal, got %v", err)
	}
}

func TestForget_ForceSucceeds(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addClaim(t, "aaa1", "agent-a", "FEAT-1", "")
	w := New(f.Store())
	if err := w.Forget(context.Background(), "aaa1", true); err != nil {
		t.Fatal(err)
	}
	rows, _ := w.List(context.Background())
	if len(rows) != 0 {
		t.Fatalf("repo should be gone, got %+v", rows)
	}
}

func TestWho_AllAgentsAcrossRepos(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addRepo(t, "bbb2", "git@github.com:a/y.git", "/y", time.Minute)
	f.addAgent(t, "aaa1", "agent-aa", "alice", 30*time.Second)
	f.addAgent(t, "bbb2", "agent-bb", "bob", time.Hour)
	f.addClaim(t, "aaa1", "agent-aa", "FEAT-1", "wiring")
	rows, _ := New(f.Store()).Who(context.Background(), Filter{})
	if len(rows) != 2 || rows[0].AgentID != "agent-aa" || rows[0].ClaimItem != "FEAT-1" {
		t.Fatalf("who wrong: %+v", rows)
	}
}

func TestList_KnownRepos(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addRepo(t, "bbb2", "git@github.com:a/y.git", "/y", 24*time.Hour)
	f.addClaim(t, "aaa1", "agent-a", "FEAT-1", "")
	rows, _ := New(f.Store()).List(context.Background())
	if len(rows) != 2 || rows[0].RepoID != "aaa1" || rows[0].ItemCount != 1 {
		t.Fatalf("list wrong: %+v", rows)
	}
}

func TestStatus_StaleThresholdExcludesOldRepos(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addRepo(t, "bbb2", "git@github.com:a/y.git", "/y", 30*24*time.Hour)
	rows, _ := New(f.Store()).Status(context.Background(),
		Filter{StaleThreshold: 7 * 24 * time.Hour})
	if len(rows) != 1 || rows[0].RepoID != "aaa1" {
		t.Fatalf("stale threshold wrong: %+v", rows)
	}
}

func TestStatus_CountsReadyAndBlockedFromMirror(t *testing.T) {
	f := newFixture(t)
	f.addRepo(t, "aaa1", "git@github.com:a/x.git", "/x", time.Minute)
	f.addItem(t, "aaa1", "FEAT-1", "P1", "ready")
	f.addItem(t, "aaa1", "FEAT-2", "P2", "ready")
	f.addItem(t, "aaa1", "BUG-9", "P0", "blocked")
	rows, _ := New(f.Store()).Status(context.Background(), Filter{})
	if rows[0].Ready != 2 || rows[0].Blocked != 1 {
		t.Fatalf("ready=%d blocked=%d", rows[0].Ready, rows[0].Blocked)
	}
}

func ids(rows []NextRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}
