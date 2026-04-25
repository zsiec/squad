package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/zsiec/squad/plugin/hooks"
)

func TestPromptHook_DefaultY(t *testing.T) {
	h := hooks.Hook{Name: "session-start", DefaultOn: true, Description: "x", TradeOff: "y"}
	got, err := promptHookWithIO(&bytes.Buffer{}, strings.NewReader("\n"), h)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("expected true (default Y), got false")
	}
}

func TestPromptHook_DefaultN(t *testing.T) {
	h := hooks.Hook{Name: "async-rewake", DefaultOn: false, Description: "x", TradeOff: "y"}
	got, err := promptHookWithIO(&bytes.Buffer{}, strings.NewReader("\n"), h)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatalf("expected false (default N), got true")
	}
}

func TestPromptHook_ExplicitYes(t *testing.T) {
	h := hooks.Hook{Name: "async-rewake", DefaultOn: false, Description: "x", TradeOff: "y"}
	got, err := promptHookWithIO(&bytes.Buffer{}, strings.NewReader("y\n"), h)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Fatalf("expected true, got false")
	}
}

func TestPromptHook_ExplicitNo(t *testing.T) {
	h := hooks.Hook{Name: "session-start", DefaultOn: true, Description: "x", TradeOff: "y"}
	got, err := promptHookWithIO(&bytes.Buffer{}, strings.NewReader("n\n"), h)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatalf("expected false, got true")
	}
}
