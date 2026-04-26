package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/zsiec/squad/plugin/hooks"
)

// TestInstallPlugin_DefaultHooksMatchEmbed regresses against the bug where
// install-plugin had a hardcoded list of three hooks (session-start,
// user-prompt-tick, pre-compact) and install-hooks --yes used a different
// set derived from plugin/hooks.embed.go. install-plugin now reads the same
// source; this test pins them together.
func TestInstallPlugin_DefaultHooksMatchEmbed(t *testing.T) {
	got := defaultOnHooks()

	want := map[string]bool{}
	for _, h := range hooks.All {
		if h.DefaultOn {
			want[h.Name] = true
		}
	}

	if len(got) == 0 {
		t.Fatal("defaultOnHooks() returned empty set; no DefaultOn=true hooks?")
	}

	for name := range want {
		if !got[name] {
			t.Errorf("install-plugin missing default-on hook %q (present in embed.go)", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("install-plugin has extra hook %q (not DefaultOn in embed.go)", name)
		}
	}

	if t.Failed() {
		gotKeys := make([]string, 0, len(got))
		for k := range got {
			gotKeys = append(gotKeys, k)
		}
		wantKeys := make([]string, 0, len(want))
		for k := range want {
			wantKeys = append(wantKeys, k)
		}
		sort.Strings(gotKeys)
		sort.Strings(wantKeys)
		t.Logf("got:  [%s]", strings.Join(gotKeys, ", "))
		t.Logf("want: [%s]", strings.Join(wantKeys, ", "))
	}
}
