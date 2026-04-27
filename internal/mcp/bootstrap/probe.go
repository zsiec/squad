// Package bootstrap drives the per-connection dashboard daemon lifecycle:
// probe the running daemon, install or upgrade as needed, run a one-shot
// welcome flow, and surface a banner in the next MCP tool response.
package bootstrap

import "context"

// ProbeResult describes the daemon advertised by GET /api/version. Zero
// value means "no daemon reachable."
type ProbeResult struct {
	Reachable  bool
	Version    string
	BinaryPath string
}

// Probe asks the running daemon (if any) for its version and binary path.
// Skeleton stub — the HTTP call lands in a follow-up.
func Probe(ctx context.Context) (ProbeResult, error) {
	return ProbeResult{}, nil
}
