// Package bootstrap drives the per-connection dashboard daemon lifecycle:
// probe the running daemon, install or upgrade as needed, run a one-shot
// welcome flow, and surface a banner in the next MCP tool response.
package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// probeBase is the URL prefix Probe and Ensure target; tests override it
// to point at an httptest.Server. Production stays on the loopback bind
// the daemon advertises by default.
var probeBase = "http://127.0.0.1:7777"

// SetProbeBaseForTest swaps the URL Probe / Ensure target. Returns a
// restore closure callers should defer. Test-only seam — production
// code uses the default loopback target.
func SetProbeBaseForTest(url string) (restore func()) {
	prev := probeBase
	probeBase = url
	return func() { probeBase = prev }
}

// probeTimeout is the per-request budget Probe gives the daemon. Short
// enough that a missing daemon doesn't stall the MCP boot, long enough
// to absorb the round-trip on a busy laptop.
var probeTimeout = 500 * time.Millisecond

// ProbeResult describes the daemon advertised by GET /api/version. Zero
// value (Present=false) means "no daemon reachable."
type ProbeResult struct {
	Present    bool
	Version    string
	BinaryPath string
	StartedAt  time.Time
	PID        int
}

// Probe asks the running daemon (if any) at /api/version for its
// version, binary path, start time and pid. A connection refusal,
// timeout, or non-200 reply is reported as Present=false with no error
// — the absence of a daemon is the normal first-run case, not a failure.
func Probe(ctx context.Context) (ProbeResult, error) {
	reqCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, probeBase+"/api/version", nil)
	if err != nil {
		return ProbeResult{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ProbeResult{}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ProbeResult{}, nil
	}
	var body struct {
		Version    string `json:"version"`
		BinaryPath string `json:"binary_path"`
		StartedAt  string `json:"started_at"`
		PID        int    `json:"pid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ProbeResult{}, err
	}
	out := ProbeResult{
		Present:    true,
		Version:    body.Version,
		BinaryPath: body.BinaryPath,
		PID:        body.PID,
	}
	if body.StartedAt != "" {
		t, err := time.Parse(time.RFC3339, body.StartedAt)
		if err != nil {
			return ProbeResult{}, err
		}
		out.StartedAt = t
	}
	return out, nil
}
