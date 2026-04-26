package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GET_AddsBearerToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := New(srv.URL, "secret-token")
	var got map[string]any
	if err := c.GET(context.Background(), "/api/health", &got); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("auth=%q", gotAuth)
	}
	if got["ok"] != true {
		t.Fatalf("body=%v", got)
	}
}

func TestClient_GET_NoTokenWhenEmpty(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	var got map[string]any
	if err := c.GET(context.Background(), "/api/health", &got); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no auth header when token empty, got %q", gotAuth)
	}
}

func TestClient_GET_ReturnsStatusErrOn404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	var got map[string]any
	err := c.GET(context.Background(), "/api/items/X", &got)
	if err == nil {
		t.Fatal("expected error")
	}
	se, ok := err.(*StatusErr)
	if !ok {
		t.Fatalf("want *StatusErr, got %T: %v", err, err)
	}
	if se.Code != 404 {
		t.Fatalf("code=%d", se.Code)
	}
	if se.Endpoint != "GET /api/items/X" {
		t.Fatalf("endpoint=%q", se.Endpoint)
	}
}

func TestClient_POST_SendsJSONBody(t *testing.T) {
	var gotBody map[string]any
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	var resp map[string]any
	if err := c.POST(context.Background(), "/api/messages", map[string]any{"thread": "global", "body": "hi"}, &resp); err != nil {
		t.Fatal(err)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type=%q", gotContentType)
	}
	if gotBody["thread"] != "global" || gotBody["body"] != "hi" {
		t.Fatalf("body=%v", gotBody)
	}
	if resp["ok"] != true {
		t.Fatalf("resp=%v", resp)
	}
}
