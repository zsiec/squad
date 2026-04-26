package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zsiec/squad/internal/attest"
)

func TestAttestations_ListForItem(t *testing.T) {
	db := newTestDB(t)
	ledger := attest.New(db, testRepoID, func() time.Time { return time.Unix(1700000000, 0) })
	if _, err := ledger.Insert(context.Background(), attest.Record{
		ItemID:     "BUG-100",
		Kind:       attest.KindTest,
		Command:    "go test ./...",
		ExitCode:   0,
		OutputHash: "abc123",
		OutputPath: ".squad/attestations/abc123.txt",
		AgentID:    "agent-test",
	}); err != nil {
		t.Fatal(err)
	}

	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-100/attestations", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 1 {
		t.Fatalf("want 1 attestation, got %d: %v", len(out), out)
	}
	if out[0]["kind"] != "test" || out[0]["command"] != "go test ./..." {
		t.Fatalf("got %v", out[0])
	}
}

func TestAttestations_EmptyItem(t *testing.T) {
	db := newTestDB(t)
	s := New(db, testRepoID, Config{SquadDir: "testdata"})
	req := httptest.NewRequest(http.MethodGet, "/api/items/BUG-100/attestations", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	var out []map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if len(out) != 0 {
		t.Fatalf("want 0 attestations, got %d", len(out))
	}
}
