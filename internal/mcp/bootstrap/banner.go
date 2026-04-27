package bootstrap

import (
	"fmt"
	"sync/atomic"
)

// banner buffers a one-shot string to surface in the next MCP tool
// response. Storing through atomic.Value gives lock-free Set + clear-on-
// consume semantics across the goroutine that runs Ensure and the
// goroutine that handles the next tool call.
var banner atomic.Value // string

// SetBanner stages a banner. The latest call wins until ConsumeBanner
// drains it.
func SetBanner(s string) {
	banner.Store(s)
}

// ConsumeBanner returns the staged banner and clears it. Returns "" if
// nothing is staged. Safe to call from any goroutine.
func ConsumeBanner() string {
	v := banner.Swap("")
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// BannerInstalled is the copy Ensure stages after a fresh daemon install.
func BannerInstalled(port int) string {
	return fmt.Sprintf("Squad dashboard ready at http://localhost:%d", port)
}

// BannerUpgraded is the copy Ensure stages after detecting and resolving
// a version skew between the daemon and the binary.
func BannerUpgraded(version string) string {
	return fmt.Sprintf("Squad upgraded to %s; dashboard restarted", version)
}

// BannerPortConflict is the copy Ensure stages when it cannot bring up
// the daemon because the configured port is already in use.
func BannerPortConflict(port int) string {
	return fmt.Sprintf("Squad dashboard unavailable: port %d in use", port)
}

// BannerUnsupported is the copy Ensure stages on platforms where auto-
// install is not implemented (Windows today). The message tells the user
// how to bring up the dashboard by hand.
const BannerUnsupported = `Squad dashboard auto-install not supported on this platform; run "squad serve" manually`
