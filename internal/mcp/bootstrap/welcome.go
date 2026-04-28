package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

const welcomeSentinel = ".welcomed"

// Welcome writes the .welcomed sentinel on first MCP boot and is a
// no-op thereafter. The auto-open-browser behavior was removed: in
// multi-agent setups the dashboard browser was re-opening mid-session,
// and although a probe against this code path showed Welcome was not
// the source, removing the auto-open here narrows the surface for that
// class of bug — the install banner already prints the URL for anyone
// who wants to see it. If the symptom persists after this change, the
// real source is elsewhere.
//
// Sentinel path: opts.SquadHome (preferred — honors SQUAD_HOME drift)
// falls back to opts.HomeDir/.squad. Stat errors other than ENOENT
// (e.g. EACCES on a chmod 000 .squad) skip with a stderr line and
// no write — bias toward silence on flaky filesystem shapes.
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
		fmt.Fprintf(os.Stderr, "squad: welcome: cannot stat sentinel %s: %v; skipping\n", sentinel, err)
		return nil
	}
	if err := writeSentinelAtomic(sentinel); err != nil {
		return fmt.Errorf("write welcome sentinel: %w", err)
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
