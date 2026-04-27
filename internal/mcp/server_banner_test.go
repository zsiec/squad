package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/mcp/bootstrap"
)

// drainBanner ensures atomic.Value-backed banner storage starts empty for
// each test that asserts consume-and-clear semantics.
func drainBanner() {
	for bootstrap.ConsumeBanner() != "" {
	}
}

func decodeCallResponse(t *testing.T, raw []byte) (content []map[string]any, isError bool, errMsg string) {
	t.Helper()
	var resp struct {
		Result *struct {
			Content []map[string]any `json:"content"`
			IsError bool             `json:"isError"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode: %v\nraw: %s", err, raw)
	}
	if resp.Error != nil {
		return nil, false, resp.Error.Message
	}
	if resp.Result == nil {
		t.Fatalf("response has neither result nor error: %s", raw)
	}
	return resp.Result.Content, resp.Result.IsError, ""
}

func TestCallTool_BannerPrependedThenConsumed(t *testing.T) {
	drainBanner()
	t.Cleanup(drainBanner)

	srv := NewServer(ServerInfo{Name: "squad", Version: "test"})
	srv.Register(Tool{
		Name:        "echo",
		Description: "test echo",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			return "ok", nil
		},
	})

	bootstrap.SetBanner("Squad dashboard ready at http://localhost:7777")

	calls := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{}}}` + "\n",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{}}}` + "\n",
	}
	var out bytes.Buffer
	if err := srv.Serve(context.Background(), strings.NewReader(strings.Join(calls, "")), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	lines := bytes.Split(bytes.TrimRight(out.Bytes(), "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d: %s", len(lines), out.String())
	}

	firstContent, _, _ := decodeCallResponse(t, lines[0])
	if len(firstContent) != 2 {
		t.Fatalf("first response should have 2 content blocks (banner + tool result), got %d: %s", len(firstContent), lines[0])
	}
	if firstContent[0]["type"] != "text" || firstContent[0]["text"] != "Squad dashboard ready at http://localhost:7777" {
		t.Fatalf("first response banner block wrong: %+v", firstContent[0])
	}

	secondContent, _, _ := decodeCallResponse(t, lines[1])
	if len(secondContent) != 1 {
		t.Fatalf("second response should have 1 content block (consumed banner), got %d: %s", len(secondContent), lines[1])
	}
}

func TestCallTool_BannerSurvivesFirstCallError(t *testing.T) {
	drainBanner()
	t.Cleanup(drainBanner)

	srv := NewServer(ServerInfo{Name: "squad", Version: "test"})
	var calls int
	srv.Register(Tool{
		Name:        "flaky",
		Description: "errors first, succeeds after",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Handler: func(ctx context.Context, args json.RawMessage) (any, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("boom")
			}
			return "ok", nil
		},
	})

	bootstrap.SetBanner("Squad upgraded to 0.3.0; dashboard restarted")

	reqs := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"flaky","arguments":{}}}` + "\n",
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"flaky","arguments":{}}}` + "\n",
	}, "")
	var out bytes.Buffer
	if err := srv.Serve(context.Background(), strings.NewReader(reqs), &out); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	lines := bytes.Split(bytes.TrimRight(out.Bytes(), "\n"), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 response lines, got %d", len(lines))
	}

	if _, _, errMsg := decodeCallResponse(t, lines[0]); errMsg == "" {
		t.Fatalf("first response should be an error, got success: %s", lines[0])
	}

	successContent, _, _ := decodeCallResponse(t, lines[1])
	if len(successContent) != 2 {
		t.Fatalf("second (successful) response should carry banner + result, got %d blocks: %s", len(successContent), lines[1])
	}
	if successContent[0]["text"] != "Squad upgraded to 0.3.0; dashboard restarted" {
		t.Fatalf("banner not preserved across error: got %+v", successContent[0])
	}
}
