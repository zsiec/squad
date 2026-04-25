package prmark

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

func clipboardCommand() []string {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("pbcopy"); err == nil {
			return []string{"pbcopy"}
		}
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return []string{"wl-copy"}
		}
		if _, err := exec.LookPath("xclip"); err == nil {
			return []string{"xclip", "-selection", "clipboard"}
		}
	case "windows":
		if _, err := exec.LookPath("clip"); err == nil {
			return []string{"clip"}
		}
	}
	return nil
}

var errNoClipboard = errors.New("no clipboard tool found (install pbcopy / xclip / wl-copy / clip)")

func WriteClipboard(s string) error {
	argv := clipboardCommand()
	if argv == nil {
		return errNoClipboard
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}
