package stats

import (
	"context"
	"database/sql"
	"os"
	"strconv"
)

const tokenEnvVar = "CLAUDE_CODE_TRANSCRIPT_BYTES"

func computeTokens(_ context.Context, _ *sql.DB, _ string, _, _ int64, snap *Snapshot) error {
	snap.Tokens.PerItemEstimateMethod = "unavailable"
	if v := os.Getenv(tokenEnvVar); v != "" {
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			// The harness exposes a per-process number, not per-item. Until
			// claim/done flows persist the per-claim delta (R3 territory),
			// surfacing the live process value would be misleading. Keep the
			// method tag visible so a future R3 wire-up flips this to
			// "transcript_bytes_env" without a schema change.
			snap.Tokens.PerItemEstimateMethod = "transcript_bytes_env_unwired"
		}
	}
	return nil
}
