package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const welcomeSentinel = ".welcomed"

// Welcome runs the first-run flow: open the dashboard in the user's
// browser and write the .welcomed sentinel so the open happens at most
// once. Failures never return an error — the MCP server must keep
// serving tools even if the welcome flow can't complete.
func Welcome(ctx context.Context, opts Options) error {
	sentinel := filepath.Join(opts.HomeDir, ".squad", welcomeSentinel)
	if _, err := os.Stat(sentinel); err == nil {
		return nil
	}

	if os.Getenv("SQUAD_NO_BROWSER") != "1" {
		open := opts.Opener
		if open == nil {
			open = defaultOpener
		}
		url := fmt.Sprintf("http://localhost:%d", opts.Port)
		if err := open(url); err != nil {
			fmt.Fprintf(os.Stderr, "squad: warning: could not auto-open dashboard at %s: %v\n", url, err)
		}
	}

	if err := writeSentinel(sentinel); err != nil {
		fmt.Fprintf(os.Stderr, "squad: warning: could not write %s sentinel: %v\n", sentinel, err)
	}
	return nil
}

func writeSentinel(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}

func defaultOpener(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("auto-open not supported on %s", runtime.GOOS)
	}
	return cmd.Start()
}
