package server

import (
	"strings"
	"testing"
)

// TestInsightsPanelRendersExpandedTileSet pins the FEAT-056 contract:
// the stats panel must render at least 8 distinct tiles backed by
// /api/stats and expose a window selector that switches 24h / 7d /
// 30d in place. Reads the embedded webFS (the same bytes the daemon
// serves), asserts each tile id is present and that the selector
// declares all three windows. Same approach as the BUG-043 CSS pin
// and BUG-052 SPA-URL pin.
func TestInsightsPanelRendersExpandedTileSet(t *testing.T) {
	body, err := webFS.ReadFile("web/insights.js")
	if err != nil {
		t.Fatalf("read embedded insights.js: %v", err)
	}
	src := string(body)

	wantTiles := []string{
		// Pre-existing tiles — must survive the redesign.
		`id="chart-verify"`,
		`id="chart-p99"`,
		`id="chart-wip"`,
		// New tiles — AC requires 8 total, of which at least these 5
		// (item-status mix, claim percentiles, verify by-kind, agent
		// leaderboard, by-capability) appear by name.
		`id="chart-status-mix"`,
		`id="chart-claim-percentiles"`,
		`id="chart-verify-bykind"`,
		`id="leaderboard-table"`,
		`id="chart-by-capability"`,
	}
	for _, tile := range wantTiles {
		if !strings.Contains(src, tile) {
			t.Errorf("insights.js missing tile id %q", tile)
		}
	}

	wantWindowOptions := []string{
		`value="24h"`,
		`value="168h"`, // 7d
		`value="720h"`, // 30d
	}
	for _, opt := range wantWindowOptions {
		if !strings.Contains(src, opt) {
			t.Errorf("insights.js missing window option %q", opt)
		}
	}

	if !strings.Contains(src, `id="insights-window"`) {
		t.Errorf("insights.js missing window selector id=\"insights-window\"")
	}

	// AC#3 empty-state: every chart-tile renders an explicit "no data"
	// path rather than a blank canvas or chart.js error. Pinning the
	// substring catches the regression class where someone removes the
	// guard during a refactor.
	if !strings.Contains(src, "no data") {
		t.Errorf(`insights.js missing "no data" empty-state copy`)
	}
}
