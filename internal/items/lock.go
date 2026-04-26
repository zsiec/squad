package items

import (
	"os"
	"path/filepath"
	"syscall"
)

// withItemsLock takes an exclusive flock on a sentinel file inside squadDir
// for the duration of fn. Used by NewWithOptions so two concurrent
// `squad new` invocations on the same machine can't both pick the same
// numeric id. The lock is per-process advisory; nothing
// stops a peer on another machine, but cross-machine races would surface
// later as git merge conflicts on the items/ directory anyway.
//
// Unix-only — squad does not target Windows. flock is a no-op fallback if
// the syscall isn't available; the worst case is the prior TOCTOU window,
// which doctor's duplicate_id finding still catches after the fact.
func withItemsLock(squadDir string, fn func() error) error {
	if err := os.MkdirAll(squadDir, 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(squadDir, ".items.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fn() // can't acquire; proceed without lock rather than fail
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fn() // flock unsupported on this fs; same fallback
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}
