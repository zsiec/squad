package views

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zsiec/squad/internal/tui/client"
)

func reposFixture(t *testing.T, repos []client.Repo) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/repos" {
			_ = json.NewEncoder(w).Encode(repos)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRepos_InitFetchesAndPopulates(t *testing.T) {
	repos := []client.Repo{
		{RepoID: "repo-a", Path: "/path/a", Remote: "git@github.com:org/a.git"},
		{RepoID: "repo-b", Path: "/path/b", Remote: ""},
	}
	srv := reposFixture(t, repos)
	c := client.New(srv.URL, "")
	m := NewRepos(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	mm := updated.(ReposModel)
	out := mm.View()
	for _, want := range []string{"repo-a", "repo-b", "/path/a"} {
		if !strings.Contains(out, want) {
			t.Errorf("View missing %q: %q", want, out)
		}
	}
}

func TestRepos_EnterEmitsScopeMsg(t *testing.T) {
	repos := []client.Repo{{RepoID: "repo-a", Path: "/p", Remote: ""}}
	srv := reposFixture(t, repos)
	c := client.New(srv.URL, "")
	m := NewRepos(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ReposModel)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from Enter")
	}
	got := cmd()
	sm, ok := got.(ReposScopeMsg)
	if !ok {
		t.Fatalf("expected ReposScopeMsg, got %T", got)
	}
	if sm.RepoID != "repo-a" {
		t.Fatalf("scope=%q", sm.RepoID)
	}
	_ = updated
}

func TestRepos_SSEItemChangedRefetches(t *testing.T) {
	srv := reposFixture(t, []client.Repo{{RepoID: "r", Path: "/p"}})
	c := client.New(srv.URL, "")
	m := NewRepos(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ReposModel)
	ev := client.Event{Kind: "item_changed", Payload: []byte(`{}`)}
	_, cmd := m.Update(ev)
	if cmd == nil {
		t.Fatal("expected refetch cmd on item_changed")
	}
	msg := cmd()
	if _, ok := msg.(reposLoadedMsg); !ok {
		t.Fatalf("expected reposLoadedMsg, got %T", msg)
	}
}

func TestRepos_RefreshMsgRefetches(t *testing.T) {
	srv := reposFixture(t, nil)
	c := client.New(srv.URL, "")
	m := NewRepos(c)
	updated, _ := m.Update(runCmd(t, m.Init()))
	m = updated.(ReposModel)
	_, cmd := m.Update(RefreshMsg{})
	if cmd == nil {
		t.Fatal("expected cmd from RefreshMsg")
	}
}
