package bootstrap

import "sync/atomic"

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
