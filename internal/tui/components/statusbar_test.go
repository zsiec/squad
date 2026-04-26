package components

import (
	"strings"
	"testing"
)

func TestStatusBar_RendersCoreFields(t *testing.T) {
	out := StatusBar(StatusBarState{
		Scope: "my-repo",
		View:  "items",
		Conn:  ConnUp,
		Width: 80,
	})
	if !strings.Contains(out, "scope:my-repo") {
		t.Errorf("missing scope: %q", out)
	}
	if !strings.Contains(out, "view:items") {
		t.Errorf("missing view: %q", out)
	}
}

func TestStatusBar_ShowsNotificationCount(t *testing.T) {
	out := StatusBar(StatusBarState{
		Scope:  "my-repo",
		View:   "items",
		Conn:   ConnUp,
		Notify: 3,
		Width:  80,
	})
	if !strings.Contains(out, "3") {
		t.Errorf("notify count not rendered: %q", out)
	}
}

func TestStatusBar_OmitsLagWhenZero(t *testing.T) {
	out := StatusBar(StatusBarState{
		Scope: "my-repo",
		View:  "items",
		Conn:  ConnUp,
		Width: 80,
	})
	if strings.Contains(out, "lag") {
		t.Errorf("lag rendered when zero: %q", out)
	}
}

func TestStatusBar_ShowsLagWhenNonZero(t *testing.T) {
	out := StatusBar(StatusBarState{
		Scope: "my-repo",
		View:  "items",
		Conn:  ConnUp,
		Lag:   250,
		Width: 80,
	})
	if !strings.Contains(out, "lag") || !strings.Contains(out, "250") {
		t.Errorf("lag not rendered: %q", out)
	}
}

func TestStatusBar_ShowsConnStateIndicator(t *testing.T) {
	// All three conn states render the dot character (●). The styling differs
	// (color), but the character should be present in any render.
	for _, state := range []ConnState{ConnUp, ConnReconnecting, ConnDown} {
		out := StatusBar(StatusBarState{
			Scope: "x",
			View:  "items",
			Conn:  state,
			Width: 80,
		})
		if !strings.Contains(out, "●") {
			t.Errorf("conn state %v: dot not rendered: %q", state, out)
		}
	}
}

func TestStatusBar_AppendsHints(t *testing.T) {
	out := StatusBar(StatusBarState{
		Scope: "x",
		View:  "items",
		Conn:  ConnUp,
		Hints: []string{"c=claim", "r=release"},
		Width: 120,
	})
	for _, h := range []string{"c=claim", "r=release"} {
		if !strings.Contains(out, h) {
			t.Errorf("hint %q missing: %q", h, out)
		}
	}
}
