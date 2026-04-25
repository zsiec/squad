package chat

import (
	"reflect"
	"testing"
)

func TestParseMentions(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"no mentions", nil},
		{"@agent-a heads up", []string{"agent-a"}},
		{"hi @agent-a and @thomas", []string{"agent-a", "thomas"}},
		{"email@example.com is not a mention", nil},
		{"@a @b @a dedup", []string{"a", "b"}},
		{"trailing punctuation @alice, hello", []string{"alice"}},
		{"newlines\n@bob mid-stream", []string{"bob"}},
	}
	for _, tc := range cases {
		got := ParseMentions(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseMentions(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
