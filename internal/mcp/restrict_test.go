package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRestrictTo_KeepsOnlyAllowedTools(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test", Version: "0.0.0"})
	for _, name := range []string{"alpha", "beta", "gamma", "delta"} {
		srv.Register(Tool{
			Name:        name,
			Description: "fixture",
			InputSchema: json.RawMessage(`{"type":"object"}`),
			Handler: func(context.Context, json.RawMessage) (any, error) {
				return nil, nil
			},
		})
	}
	srv.RestrictTo([]string{"alpha", "gamma"})

	srv.mu.RLock()
	defer srv.mu.RUnlock()
	if len(srv.tools) != 2 {
		t.Fatalf("after RestrictTo got %d tools, want 2", len(srv.tools))
	}
	for _, want := range []string{"alpha", "gamma"} {
		if _, ok := srv.tools[want]; !ok {
			t.Errorf("missing kept tool %q", want)
		}
	}
	for _, dropped := range []string{"beta", "delta"} {
		if _, ok := srv.tools[dropped]; ok {
			t.Errorf("dropped tool %q should not survive RestrictTo", dropped)
		}
	}
}

func TestRestrictTo_EmptyAllowlistIsNoOp(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test", Version: "0.0.0"})
	srv.Register(Tool{Name: "alpha", InputSchema: json.RawMessage(`{}`)})
	srv.RestrictTo(nil)
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	if _, ok := srv.tools["alpha"]; !ok {
		t.Errorf("RestrictTo(nil) must not touch the registry")
	}
}

func TestRestrictTo_AllowedNameNotRegisteredIsNoOp(t *testing.T) {
	srv := NewServer(ServerInfo{Name: "test", Version: "0.0.0"})
	srv.Register(Tool{Name: "alpha", InputSchema: json.RawMessage(`{}`)})
	srv.RestrictTo([]string{"alpha", "ghost"})
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	if _, ok := srv.tools["alpha"]; !ok {
		t.Errorf("kept tool dropped")
	}
	if _, ok := srv.tools["ghost"]; ok {
		t.Errorf("non-registered allowed name must not be invented")
	}
}
