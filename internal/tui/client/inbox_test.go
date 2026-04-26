package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Inbox(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		_, _ = w.Write([]byte(`[
			{"id":"FEAT-001","title":"first","captured_by":"alice","captured_at":1700000000,"parent_spec":"intake","dor_pass":true,"path":"/p/FEAT-001.md"},
			{"id":"BUG-002","title":"second","dor_pass":false,"path":"/p/BUG-002.md"}
		]`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	got, err := c.Inbox(context.Background(), InboxOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if gotURL != "/api/inbox" {
		t.Fatalf("url=%q", gotURL)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].ID != "FEAT-001" || got[0].CapturedBy != "alice" || got[0].CapturedAt != 1700000000 || got[0].ParentSpec != "intake" || !got[0].DoRPass || got[0].Path != "/p/FEAT-001.md" {
		t.Fatalf("entry[0]=%+v", got[0])
	}
	if got[1].ID != "BUG-002" || got[1].DoRPass {
		t.Fatalf("entry[1]=%+v", got[1])
	}
}

func TestClient_Accept(t *testing.T) {
	var gotURL, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	if err := c.Accept(context.Background(), "FEAT-001"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotURL != "/api/items/FEAT-001/accept" {
		t.Fatalf("method=%s url=%q", gotMethod, gotURL)
	}
}

func TestClient_Accept_DoRViolation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"violations": []map[string]string{
				{"rule": "area-set", "field": "area", "message": "area is unset"},
				{"rule": "acceptance-criterion", "field": "body", "message": "no AC checkbox"},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	err := c.Accept(context.Background(), "FEAT-001")
	if err == nil {
		t.Fatal("expected error")
	}
	var dorErr *DoRViolationsError
	if !errors.As(err, &dorErr) {
		t.Fatalf("want *DoRViolationsError, got %T: %v", err, err)
	}
	if len(dorErr.Violations) != 2 {
		t.Fatalf("violations len=%d", len(dorErr.Violations))
	}
	if dorErr.Violations[0].Rule != "area-set" || dorErr.Violations[0].Field != "area" {
		t.Fatalf("violations[0]=%+v", dorErr.Violations[0])
	}
	if dorErr.Violations[1].Rule != "acceptance-criterion" {
		t.Fatalf("violations[1]=%+v", dorErr.Violations[1])
	}
}

func TestClient_Accept_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"item not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	err := c.Accept(context.Background(), "FEAT-001")
	if err == nil {
		t.Fatal("expected error")
	}
	var dorErr *DoRViolationsError
	if errors.As(err, &dorErr) {
		t.Fatalf("404 should not be DoRViolationsError")
	}
}

func TestClient_Reject(t *testing.T) {
	var gotURL, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	if err := c.Reject(context.Background(), "FEAT-001", "duplicate"); err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotURL != "/api/items/FEAT-001/reject" {
		t.Fatalf("method=%s url=%q", gotMethod, gotURL)
	}
	if gotBody["reason"] != "duplicate" {
		t.Fatalf("body=%v", gotBody)
	}
}

func TestClient_Reject_Conflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"item is claimed"}`, http.StatusConflict)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	err := c.Reject(context.Background(), "FEAT-001", "dup")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() == "" {
		t.Fatalf("error message empty")
	}
}

func TestClient_NewItem(t *testing.T) {
	var gotURL, gotMethod string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RequestURI()
		gotMethod = r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"FEAT-001","status":"captured","path":"/p/FEAT-001.md"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	out, err := c.NewItem(context.Background(), NewItemArgs{
		Type:       "feature",
		Title:      "do the thing",
		Priority:   "P2",
		Area:       "tui",
		CapturedBy: "alice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotURL != "/api/items" {
		t.Fatalf("method=%s url=%q", gotMethod, gotURL)
	}
	if gotBody["type"] != "feature" || gotBody["title"] != "do the thing" {
		t.Fatalf("body=%v", gotBody)
	}
	if gotBody["captured_by"] != "alice" || gotBody["area"] != "tui" || gotBody["priority"] != "P2" {
		t.Fatalf("body=%v", gotBody)
	}
	if out == nil || out.ID != "FEAT-001" || out.Status != "captured" || out.Path != "/p/FEAT-001.md" {
		t.Fatalf("out=%+v", out)
	}
}
