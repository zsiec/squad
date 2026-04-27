//go:build darwin || linux

package bootstrap

import (
	"os"
	"path/filepath"
	"syscall"
)

// acquireInstallLock takes an exclusive flock on path, creating the file
// (and any missing parent dirs) on demand. Concurrent MCP sessions on a
// fresh box would otherwise race their daemon installs against each
// other; the loser blocks here until the winner finishes, then sees the
// daemon up via the next probe and treats it as a no-op.
func acquireInstallLock(path string) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
