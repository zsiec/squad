package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// TestProtocolVersion guards against drifting back to a fabricated date.
// Real MCP spec dates: 2024-11-05, 2025-03-26, 2025-06-18. Squad targets the
// latest stable release; bumping is fine, but the value must be a real spec
// date — Claude Code rejects fabricated versions during the handshake.
func TestProtocolVersion(t *testing.T) {
	known := map[string]bool{
		"2024-11-05": true,
		"2025-03-26": true,
		"2025-06-18": true,
	}
	if !known[ProtocolVersion] {
		t.Fatalf("ProtocolVersion = %q, must be one of the published MCP spec dates: %v",
			ProtocolVersion, []string{"2024-11-05", "2025-03-26", "2025-06-18"})
	}
}

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

func TestServer_NotificationProducesNoResponse(t *testing.T) {
	// Per JSON-RPC 2.0, a request without an id is a notification — the
	// server MUST NOT respond. Claude Code sends notifications/initialized
	// after the initialize handshake; if we reply, the client treats it as
	// an unknown response id and drops the stdio transport.
	srv := NewServer(ServerInfo{Name: "squad"})
	req := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n")
	var out bytes.Buffer
	if err := srv.Serve(context.Background(), req, &out); err != nil && err != io.EOF {
		t.Fatalf("Serve: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no response to a notification, got %q", out.String())
	}
}

func TestServer_NotificationWithExplicitNullIDProducesNoResponse(t *testing.T) {
	// Edge case: id explicitly set to null is also a notification per spec.
	srv := NewServer(ServerInfo{Name: "squad"})
	req := strings.NewReader(`{"jsonrpc":"2.0","id":null,"method":"notifications/initialized"}` + "\n")
	var out bytes.Buffer
	_ = srv.Serve(context.Background(), req, &out)
	if out.Len() != 0 {
		t.Fatalf("expected no response, got %q", out.String())
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
