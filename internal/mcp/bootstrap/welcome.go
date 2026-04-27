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

// Welcome runs the first-run flow: write the .welcomed sentinel and
// then open the dashboard in the user's browser. The sentinel comes
// FIRST so the browser opens at most once per machine even if the
// opener crashes or the process is killed mid-flight; an opener error
// is logged but never repeated. A sentinel-write failure is the only
// error Welcome returns — without that signal the caller cannot tell
// whether the next session will silently re-open Chrome.
//
// Sentinel path: opts.SquadHome (preferred — honors SQUAD_HOME drift)
// falls back to opts.HomeDir/.squad. Stat errors other than ENOENT (e.g.
// EACCES on a chmod 000 .squad) skip the opener with a stderr line: the
// caller has likely completed welcome and the sentinel is just unreadable
// from this session, so re-popping the browser would be the wrong default.
func Welcome(ctx context.Context, opts Options) error {
	squadHome := opts.SquadHome
	if squadHome == "" {
		squadHome = filepath.Join(opts.HomeDir, ".squad")
	}
	sentinel := filepath.Join(squadHome, welcomeSentinel)
	switch _, err := os.Stat(sentinel); {
	case err == nil:
		return nil
	case !os.IsNotExist(err):
		fmt.Fprintf(os.Stderr, "squad: welcome: cannot stat sentinel %s: %v; skipping auto-open\n", sentinel, err)
		return nil
	}

	if err := writeSentinelAtomic(sentinel); err != nil {
		return fmt.Errorf("write welcome sentinel: %w", err)
	}

	if os.Getenv("SQUAD_NO_BROWSER") == "1" {
		return nil
	}
	open := opts.Opener
	if open == nil {
		open = defaultOpener
	}
	url := fmt.Sprintf("http://localhost:%d", opts.Port)
	if err := open(url); err != nil {
		fmt.Fprintf(os.Stderr, "squad: warning: could not auto-open dashboard at %s: %v\n", url, err)
	}
	return nil
}

// writeSentinelAtomic writes path via temp+rename so a partial failure
// (interrupted write, full disk, sandboxed filesystem) cannot leave a
// half-written file that a later Welcome misreads as "already shown."
func writeSentinelAtomic(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".welcomed.tmp.*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
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
