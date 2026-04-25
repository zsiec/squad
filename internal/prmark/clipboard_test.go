package prmark

import (
	"runtime"
	"testing"
)

func TestClipboardCommandPicksPlatformBinary(t *testing.T) {
	got := clipboardCommand()
	if got == nil {
		t.Skip("no clipboard command on this platform — acceptable")
	}
	if runtime.GOOS == "darwin" && got[0] != "pbcopy" {
		t.Fatalf("darwin should use pbcopy, got %v", got)
	}
	if got[0] == "" {
		t.Fatalf("returned empty command")
	}
}
