---
id: CHORE-019
title: stats panel renderInsights races on rapid window flips — last-fetch-wins shows stale data
type: chore
priority: P2
area: web
status: captured
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-28
updated: 2026-04-28
captured_by: agent-afcd
captured_at: 1777345899
accepted_by: ""
accepted_at: 0
references: []
relates-to: []
blocked-by: []
---

## Problem

`renderInsights` (`internal/server/web/insights.js`) has no
in-flight cancellation token. On rapid window-selector flips
(e.g. user clicks 24h → 7d → 30d in quick succession), the older
`fetchJSON('/api/stats?window=…')` calls can resolve AFTER the
newer ones, so the panel renders charts for whichever fetch
landed last rather than the currently-selected window. The
selector reflects the user's pick but the data shown can be
stale until they interact again.

## Context

Source of the race (`internal/server/web/insights.js` `renderInsights`):

```js
export async function renderInsights(container) {
  destroyCharts();
  container.innerHTML = `<div class="insights-loading">loading…</div>`;
  try {
    const [snap] = await Promise.all([
      fetchJSON('/api/stats?window=' + encodeURIComponent(currentWindow)),
      ensureChartJs(),
    ]);
    container.innerHTML = TPL;
    // ...drawX(snap.X) for every tile
  } catch (err) {
    // ...
  }
}
```

Two concurrent calls A (older) and B (newer) both hit
`container.innerHTML = TPL` after their `await` resolves;
whichever resolves second wins. `destroyCharts()` is called
synchronously at the top of B but the chart instances A
constructs after its `await` are pushed onto the shared
`charts` array regardless — they remain holding handlers until
the next `destroyCharts` runs.

Surfaced by code review on FEAT-056. Not blocking that item:
the race only manifests on rapid clicks, never produces stuck
charts or crashes, and recovers on the next user interaction.

## Acceptance criteria

- [ ] `renderInsights` captures a render-token sentinel before
      its `await` and bails out post-await if the module-scoped
      token has changed (a newer call has started).
- [ ] Rapid window-selector flips (24h → 7d → 30d clicked
      faster than the fetch settles) produce the LATEST
      window's data on screen, not whichever fetch resolved
      last.
- [ ] No stale chart instances accumulate across flips —
      `charts` array does not grow unboundedly.

## Notes

Cheap fix:

```js
let renderToken = {};

export async function renderInsights(container) {
  const myToken = (renderToken = {});
  destroyCharts();
  container.innerHTML = `<div class="insights-loading">loading…</div>`;
  try {
    const [snap] = await Promise.all([...]);
    if (renderToken !== myToken) return; // stale render — newer one in flight
    container.innerHTML = TPL;
    // ...
  } catch (err) {
    if (renderToken !== myToken) return;
    container.innerHTML = `<div class="insights-error">…</div>`;
  }
}
```

No backend or schema change. Test would simulate two concurrent
calls (mock `fetchJSON` to delay) and assert the second wins.
