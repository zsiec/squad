package learning

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"
)

func TestIsNotExist_RecognizesWrappedSentinel(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"direct sentinel", fs.ErrNotExist, true},
		{"wrapped sentinel", fmt.Errorf("walk %s: %w", "/nope", fs.ErrNotExist), true},
		{"unrelated error", errors.New("permission denied"), false},
	}
	for _, c := range cases {
		if got := isNotExist(c.err); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
