package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helper to assert the client called the URL we expected
func newCapturingServer(t *testing.T, body string) (*httptest.Server, *string) {
	t.Helper()
	gotURL := new(string)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotURL = r.URL.RequestURI()
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, gotURL
}

func TestItems_HitsCorrectURL(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[{"id":"BUG-100","title":"x","status":"open"}]`)
	c := New(srv.URL, "")
	got, err := c.Items(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/items" {
		t.Fatalf("url=%q", *gotURL)
	}
	if len(got) != 1 || got[0].ID != "BUG-100" {
		t.Fatalf("got %+v", got)
	}
}

func TestItems_AppendsStatusFilter(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[]`)
	c := New(srv.URL, "")
	if _, err := c.Items(context.Background(), &ItemListOpts{Status: "open"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*gotURL, "status=open") {
		t.Fatalf("url=%q", *gotURL)
	}
}

func TestItem_DetailHitsCorrectURL(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `{"id":"BUG-100","title":"x","body_markdown":"# hi"}`)
	c := New(srv.URL, "")
	got, err := c.Item(context.Background(), "BUG-100")
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/items/BUG-100" {
		t.Fatalf("url=%q", *gotURL)
	}
	if got.ID != "BUG-100" || got.BodyMarkdown != "# hi" {
		t.Fatalf("got %+v", got)
	}
}

func TestAgents(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[{"agent_id":"a-1","display_name":"alice"}]`)
	c := New(srv.URL, "")
	got, err := c.Agents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/agents" {
		t.Fatalf("url=%q", *gotURL)
	}
	if len(got) != 1 || got[0].AgentID != "a-1" {
		t.Fatalf("got %+v", got)
	}
}

func TestSpecs(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[{"name":"s-1","title":"Spec One"}]`)
	c := New(srv.URL, "")
	got, err := c.Specs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/specs" || len(got) != 1 || got[0].Name != "s-1" {
		t.Fatalf("url=%q got=%+v", *gotURL, got)
	}
}

func TestSpec(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `{"name":"s-1","title":"Spec One","body_markdown":"# x"}`)
	c := New(srv.URL, "")
	got, err := c.Spec(context.Background(), "s-1")
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/specs/s-1" || got.Name != "s-1" {
		t.Fatalf("url=%q got=%+v", *gotURL, got)
	}
}

func TestEpics_FiltersBySpec(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[]`)
	c := New(srv.URL, "")
	if _, err := c.Epics(context.Background(), &EpicListOpts{Spec: "s-1"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*gotURL, "spec=s-1") {
		t.Fatalf("url=%q", *gotURL)
	}
}

func TestEpic(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `{"name":"e-1","spec":"s-1"}`)
	c := New(srv.URL, "")
	got, err := c.Epic(context.Background(), "e-1")
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/epics/e-1" || got.Name != "e-1" {
		t.Fatalf("url=%q got=%+v", *gotURL, got)
	}
}

func TestStats_PassesWindow(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `{"schema_version":1}`)
	c := New(srv.URL, "")
	if _, err := c.Stats(context.Background(), 86400); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*gotURL, "window=86400") {
		t.Fatalf("url=%q", *gotURL)
	}
}

func TestLearnings_AppliesFilters(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[]`)
	c := New(srv.URL, "")
	if _, err := c.Learnings(context.Background(), &LearningListOpts{State: "approved", Kind: "gotcha", Area: "server"}); err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{"state=approved", "kind=gotcha", "area=server"} {
		if !strings.Contains(*gotURL, frag) {
			t.Fatalf("url=%q missing %q", *gotURL, frag)
		}
	}
}

func TestLearning(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `{"slug":"x","title":"X","body_markdown":"y"}`)
	c := New(srv.URL, "")
	got, err := c.Learning(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/learnings/x" || got.Slug != "x" {
		t.Fatalf("url=%q got=%+v", *gotURL, got)
	}
}

func TestAttestations(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[]`)
	c := New(srv.URL, "")
	if _, err := c.Attestations(context.Background(), "BUG-100"); err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/items/BUG-100/attestations" {
		t.Fatalf("url=%q", *gotURL)
	}
}

func TestMessages_AppliesFilters(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[]`)
	c := New(srv.URL, "")
	if _, err := c.Messages(context.Background(), &MessagesOpts{Thread: "global", Limit: 50, Since: 1700000000}); err != nil {
		t.Fatal(err)
	}
	for _, frag := range []string{"thread=global", "limit=50", "since=1700000000"} {
		if !strings.Contains(*gotURL, frag) {
			t.Fatalf("url=%q missing %q", *gotURL, frag)
		}
	}
}

func TestPostMessage(t *testing.T) {
	srv, _ := newCapturingServer(t, `{"ok":true}`)
	c := New(srv.URL, "")
	if err := c.PostMessage(context.Background(), &PostMessageReq{Thread: "global", Body: "hi"}); err != nil {
		t.Fatal(err)
	}
}

func TestWhoami(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `{"agent_id":"a-1","display_name":"alice"}`)
	c := New(srv.URL, "")
	got, err := c.Whoami(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/whoami" || got.AgentID != "a-1" {
		t.Fatalf("url=%q got=%+v", *gotURL, got)
	}
}

func TestRepos(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `[{"repo_id":"r-1","path":"/path/to/squad","remote":"git@..."}]`)
	c := New(srv.URL, "")
	got, err := c.Repos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/repos" || len(got) != 1 || got[0].RepoID != "r-1" {
		t.Fatalf("url=%q got=%+v", *gotURL, got)
	}
}

func TestWorkspaceStatus(t *testing.T) {
	srv, gotURL := newCapturingServer(t, `{}`)
	c := New(srv.URL, "")
	if _, err := c.WorkspaceStatus(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *gotURL != "/api/workspace/status" {
		t.Fatalf("url=%q", *gotURL)
	}
}

func TestClaim_HitsCorrectURL(t *testing.T) {
	var gotURL string
	var gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.Claim(context.Background(), "BUG-100", &ClaimReq{Intent: "fixing", Long: false}); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotURL != "/api/items/BUG-100/claim" {
		t.Fatalf("method=%s url=%q", gotMethod, gotURL)
	}
	if gotBody["intent"] != "fixing" {
		t.Fatalf("body=%v", gotBody)
	}
}

func TestRelease_HitsCorrectURL(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.Release(context.Background(), "BUG-100", "released"); err != nil {
		t.Fatal(err)
	}
	if gotURL != "/api/items/BUG-100/release" {
		t.Fatalf("url=%q", gotURL)
	}
}

func TestDone_HitsCorrectURL(t *testing.T) {
	var gotURL string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.Done(context.Background(), "BUG-100", true); err != nil {
		t.Fatal(err)
	}
	if gotURL != "/api/items/BUG-100/done" {
		t.Fatalf("url=%q", gotURL)
	}
	if gotBody["evidence_force"] != true {
		t.Fatalf("body=%v", gotBody)
	}
}

func TestBlocked_HitsCorrectURL(t *testing.T) {
	var gotURL string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.Blocked(context.Background(), "BUG-100", "waiting on infra"); err != nil {
		t.Fatal(err)
	}
	if gotURL != "/api/items/BUG-100/blocked" {
		t.Fatalf("url=%q", gotURL)
	}
	if gotBody["reason"] != "waiting on infra" {
		t.Fatalf("body=%v", gotBody)
	}
}

func TestHandoff_HitsCorrectURL(t *testing.T) {
	var gotURL string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.Handoff(context.Background(), "BUG-100", &HandoffReq{To: "agent-other", Summary: "ctx for next"}); err != nil {
		t.Fatal(err)
	}
	if gotURL != "/api/items/BUG-100/handoff" {
		t.Fatalf("url=%q", gotURL)
	}
	if gotBody["to"] != "agent-other" {
		t.Fatalf("body=%v", gotBody)
	}
}

func TestTouch_HitsCorrectURL(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.Touch(context.Background(), "BUG-100"); err != nil {
		t.Fatal(err)
	}
	if gotURL != "/api/items/BUG-100/touch" {
		t.Fatalf("url=%q", gotURL)
	}
}

func TestForceRelease_HitsCorrectURL(t *testing.T) {
	var gotURL string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true,"prior_holder":"agent-old"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	resp, err := c.ForceRelease(context.Background(), "BUG-100", "stale")
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "/api/items/BUG-100/force-release" {
		t.Fatalf("url=%q", gotURL)
	}
	if gotBody["reason"] != "stale" {
		t.Fatalf("body=%v", gotBody)
	}
	if resp.PriorHolder != "agent-old" {
		t.Fatalf("prior_holder=%q", resp.PriorHolder)
	}
}

func TestPostMessage_PassesKind(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	if err := c.PostMessage(context.Background(), &PostMessageReq{Thread: "BUG-100", Body: "?", Kind: "ask"}); err != nil {
		t.Fatal(err)
	}
	if gotBody["kind"] != "ask" {
		t.Fatalf("kind=%v", gotBody["kind"])
	}
}
