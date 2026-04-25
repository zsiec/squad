package listener

import "testing"

func TestProbe_LoopbackBindWorksOnDevHost(t *testing.T) {
	if !ProbeLoopbackBind() {
		t.Skip("dev host cannot bind 127.0.0.1:0; skipping (this is the sandboxed-env signal)")
	}
}

func TestProbe_ReportsErrOnBlockedAddr(t *testing.T) {
	ok, err := probeBindAddr("invalid-host:99999")
	if ok {
		t.Fatal("expected bind to fail on invalid addr")
	}
	if err == nil {
		t.Fatal("expected non-nil err on bind failure")
	}
}
