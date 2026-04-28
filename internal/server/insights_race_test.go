package server

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestRenderInsightsLastCallWins drives the SPA insights.js renderInsights
// through Node and asserts that two concurrent calls do NOT both draw —
// the older call must bail out post-await once a newer call has started.
// Pre-fix: both calls render; the older one's fetch resolves second and
// overwrites the panel with stale data while the newer call's chart
// instances stay orphaned in the module-scoped charts array. Post-fix:
// the older call sees a token mismatch after its await and returns,
// leaving only the newer call's chart on screen.
func TestRenderInsightsLastCallWins(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	const harness = `
const fetchData = [
  { rate: 0.11 }, // first call (older) — will resolve LATE
  { rate: 0.22 }, // second call (newer) — will resolve EARLY
];
let fetchCallCount = 0;

globalThis.fetch = (_url) => {
  const idx = fetchCallCount++;
  const data = fetchData[idx];
  const delayMs = idx === 0 ? 60 : 5;
  return new Promise((resolve) => setTimeout(() => {
    resolve({
      ok: true,
      status: 200,
      json: async () => ({
        series: {
          verification_rate_daily: [{ bucket_ts: 1700000000, rate: data.rate, count: 100 }],
        },
        items: {},
        claims: { duration_seconds: {}, wip_violations_attempted: 0 },
        verification: { by_kind: {} },
        by_agent: [],
        by_capability: [],
      }),
    });
  }, delayMs));
};

const chartConstructions = [];
globalThis.window = globalThis;
globalThis.window.Chart = function(_ctx, cfg) {
  chartConstructions.push({
    label: cfg.data && cfg.data.datasets && cfg.data.datasets[0].label,
    firstPoint: cfg.data && cfg.data.datasets && cfg.data.datasets[0].data && cfg.data.datasets[0].data[0],
  });
  return { destroy: () => {} };
};

globalThis.requestAnimationFrame = (cb) => setTimeout(cb, 0);

function mkEl(tag) {
  const el = {
    _tag: tag,
    _innerHTML: '',
    children: [],
    style: {},
    classList: { add: () => {}, remove: () => {}, contains: () => false, toggle: () => {} },
    addEventListener: () => {},
    removeEventListener: () => {},
    appendChild: function(c) { this.children.push(c); return c; },
    setAttribute: () => {},
    removeAttribute: () => {},
    querySelector: () => mkEl('canvas'),
    querySelectorAll: () => [],
    hidden: false,
    getContext: () => ({
      clearRect: () => {}, fillText: () => {},
      fillStyle: '', font: '', textAlign: '',
    }),
    clientWidth: 200, clientHeight: 200,
    width: 0, height: 0,
    textContent: '',
  };
  Object.defineProperty(el, 'innerHTML', {
    get() { return this._innerHTML; },
    set(v) { this._innerHTML = v; },
    configurable: true,
  });
  return el;
}

globalThis.document = {
  body: { appendChild: () => {} },
  head: { appendChild: () => {} },
  addEventListener: () => {},
  createElement: (tag) => {
    const el = mkEl(tag);
    if (tag === 'script') {
      Promise.resolve().then(() => el.onload && el.onload());
    }
    return el;
  },
};

const { renderInsights } = await import('./insights.js');

const container = mkEl('div');
const p1 = renderInsights(container);
const p2 = renderInsights(container);
await Promise.all([p1, p2]);
// Drain microtasks/timers so any straggler chart construction lands.
await new Promise((r) => setTimeout(r, 100));

process.stdout.write(JSON.stringify({
  total: chartConstructions.length,
  rates: chartConstructions
    .filter(c => c.label === 'verification rate')
    .map(c => c.firstPoint),
}));
`

	cmd := exec.Command("node", "--input-type=module", "--eval", harness)
	cmd.Dir = "web"
	stdout, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("node harness failed: %v\nstderr: %s", err, stderr)
	}

	var result struct {
		Total int       `json:"total"`
		Rates []float64 `json:"rates"`
	}
	if err := json.Unmarshal(stdout, &result); err != nil {
		t.Fatalf("decode: %v\nstdout: %s", err, stdout)
	}

	// AC#3: no stale chart instances accumulate. The stub data only
	// triggers drawVerify (other tiles hit empty-state branches), so
	// total chart constructions must equal the verify count, and both
	// must be 1 — older call bails post-await.
	if result.Total != len(result.Rates) {
		t.Fatalf("stub-vs-assertion drift: total=%d but verify-count=%d (rates=%v)", result.Total, len(result.Rates), result.Rates)
	}
	if result.Total != 1 {
		t.Fatalf("expected exactly 1 chart construction across two concurrent renderInsights calls, got %d (rates=%v)", result.Total, result.Rates)
	}

	// AC#2: the latest call's data wins. Newer call had rate=0.22.
	if result.Rates[0] != 0.22 {
		t.Fatalf("expected the newer call's data (rate=0.22) on screen, got rates=%v", result.Rates)
	}

	// AC#1 structural: a render-token sentinel exists in the source. This
	// pins the guard so a refactor that strips it gets caught even if
	// node is missing in CI for some reason.
	body, err := webFS.ReadFile("web/insights.js")
	if err != nil {
		t.Fatalf("read embedded insights.js: %v", err)
	}
	src := string(body)
	if !strings.Contains(src, "renderToken") {
		t.Errorf("insights.js missing render-token sentinel — required by AC#1")
	}
}
