package hooks

import (
	"testing"
)

func TestAll_IncludesNewR1Hooks(t *testing.T) {
	want := map[string]string{
		"stop-listen":         "Stop",
		"post-tool-flush":     "PostToolUse",
		"session-end-cleanup": "SessionEnd",
		"async-rewake":        "asyncRewake",
	}
	got := map[string]string{}
	for _, h := range All {
		got[h.Name] = h.EventType
	}
	for name, ev := range want {
		if got[name] != ev {
			t.Errorf("hook %q: event=%q want %q", name, got[name], ev)
		}
	}
}

func TestAll_StopListenIsDefaultOn(t *testing.T) {
	for _, h := range All {
		if h.Name == "stop-listen" && !h.DefaultOn {
			t.Fatal("stop-listen must be default-on")
		}
	}
}

func TestAll_AsyncRewakeIsDefaultOff(t *testing.T) {
	for _, h := range All {
		if h.Name == "async-rewake" && h.DefaultOn {
			t.Fatal("async-rewake must be opt-in (default off)")
		}
	}
}
