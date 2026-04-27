// Package daemon installs squad serve as a per-user system service —
// launchd on darwin, systemd-user on linux. The TUI calls into Manager
// on first launch so a fresh box can run `squad tui` and end up with a
// working background daemon plus a connected client in one step.
package daemon

import "errors"

// Manager is the platform-specific service controller. New() returns
// the right implementation for the host OS; tests inject their own.
type Manager interface {
	Install(opts InstallOpts) error
	Uninstall() error
	Status() (Status, error)
	Reinstall(opts InstallOpts) error
}

// InstallOpts captures everything the platform-specific service file
// templates need. BinaryPath is whatever os.Executable() resolved to so
// the service starts the same binary the installer ran from.
type InstallOpts struct {
	BinaryPath string
	Bind       string
	Port       int
	LogDir     string
	HomeDir    string
}

// Status reports what the manager can determine without parsing
// platform-specific output beyond what's needed to answer
// "is it installed and running?".
type Status struct {
	Installed  bool
	Running    bool
	PID        int
	BinaryPath string
	Bind       string
}

// ErrUnsupported is returned by the unsupported-OS stub. Other Managers
// shouldn't return it.
var ErrUnsupported = errors.New("daemon install not supported on this platform")
