package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestServer_RespondsToInitialize(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "squad", Version: "test"})
	req := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}` + "\n")
	var out bytes.Buffer
	if err := srv.Serve(context.Background(), req, &out); err != nil && err != io.EOF {
		t.Fatalf("Serve: %v", err)
	}
	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, out.String())
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc: got %q", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("id: got %d", resp.ID)
	}
	if resp.Result.ServerInfo.Name != "squad" {
		t.Errorf("server name: got %q", resp.Result.ServerInfo.Name)
	}
}

func TestServer_UnknownMethodReturnsMethodNotFound(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "squad"})
	req := strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"nope","params":{}}` + "\n")
	var out bytes.Buffer
	_ = srv.Serve(context.Background(), req, &out)

	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(out.Bytes(), &resp)
	if resp.Error.Code != -32601 {
		t.Errorf("expected method-not-found (-32601), got %d", resp.Error.Code)
	}
}

func TestServer_InvalidJSONReturnsParseError(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "squad"})
	req := strings.NewReader("{not json\n")
	var out bytes.Buffer
	_ = srv.Serve(context.Background(), req, &out)

	var resp struct {
		ID    json.RawMessage `json:"id"`
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, out.String())
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected parse-error (-32700), got %d", resp.Error.Code)
	}
	// JSON-RPC 2.0 §5.1: parse-error responses MUST include id=null.
	if got := string(resp.ID); got != "null" {
		t.Errorf("id field = %q, want \"null\" (raw response: %s)", got, out.String())
	}
}
