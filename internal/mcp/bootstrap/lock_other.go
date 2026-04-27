//go:build !darwin && !linux

package bootstrap

// acquireInstallLock is a no-op on platforms where the daemon installer
// itself returns ErrUnsupported — Ensure will fail at Manager.Install
// before any race over the lock matters.
func acquireInstallLock(path string) (release func(), err error) {
	return func() {}, nil
}
