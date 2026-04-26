//go:build !darwin && !linux

package daemon

// New returns a Manager whose every method returns ErrUnsupported. The
// TUI's first-launch flow detects this and surfaces a clear error rather
// than silently calling no-op methods.
func New() Manager { return unsupportedManager{} }

type unsupportedManager struct{}

func (unsupportedManager) Install(InstallOpts) error   { return ErrUnsupported }
func (unsupportedManager) Uninstall() error            { return ErrUnsupported }
func (unsupportedManager) Status() (Status, error)     { return Status{}, ErrUnsupported }
func (unsupportedManager) Reinstall(InstallOpts) error { return ErrUnsupported }
