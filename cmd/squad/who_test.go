package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunWho_PrintsHeader(t *testing.T) {
	f := newChatFixture(t)
	var buf bytes.Buffer
	if code := runWhoBody(context.Background(), f.chat, false, false, &buf); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(buf.String(), "AGENT") {
		t.Fatalf("missing header: %q", buf.String())
	}
}

func TestRunWho_JSONFormat(t *testing.T) {
	f := newChatFixture(t)
	var buf bytes.Buffer
	if code := runWhoBody(context.Background(), f.chat, true, false, &buf); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "[") {
		t.Fatalf("expected JSON array, got %q", out)
	}
}
