package learning

import (
	"testing"
)

func TestRewriteState_ReplacesLineRegardlessOfTrailing(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "---\nstate: proposed\n---\n", "---\nstate: approved\n---\n"},
		{"trailing-spaces", "---\nstate: proposed   \n---\n", "---\nstate: approved\n---\n"},
		{"already-target", "---\nstate: approved\n---\n", "---\nstate: approved\n---\n"},
	}
	for _, c := range cases {
		got := string(RewriteState([]byte(c.in), StateApproved))
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
