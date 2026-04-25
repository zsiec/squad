package listener

import "net"

// ProbeLoopbackBind returns true iff the process can bind 127.0.0.1:0.
// Used at install time to decide whether to install the Stop-listen hook
// or fall back to the legacy `tick`-based polling path. Sandboxed
// environments (some CI runners, container security policies) deny
// loopback bind; squad must not silently install a hook that will fail
// every session on those hosts.
func ProbeLoopbackBind() bool {
	ok, _ := probeBindAddr("127.0.0.1:0")
	return ok
}

func probeBindAddr(addr string) (bool, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return false, err
	}
	_ = l.Close()
	return true, nil
}
