---
id: BUG-016
title: web SPA nav button blanks the page — nav-detail position:absolute inset:0 leaks past hidden
type: bug
priority: P1
area: web-ui
status: open
estimate: 15m
risk: low
created: 2026-04-26
updated: 2026-04-26
captured_by: agent-401f
captured_at: 1777242496
accepted_by: agent-401f
accepted_at: 1777242496
references: []
relates-to: []
blocked-by: []
---

## Problem

Clicking the NAV button in the topbar opens the navigation sidebar AND covers the entire viewport with a blank panel. The board, chat, and topbar buttons become unreachable; the only escape is to close NAV via shortcut.

## Context

User-reported following BUG-004. Verified via Playwright + headless Chrome — clicking `#nav-toggle-btn` produces a fully blank dark screen.

Root cause is in `internal/server/web/style.css`:

```css
.nav-detail {
  position: absolute;
  inset: 0;
  background: var(--panel);
  z-index: 5;
  display: flex; ...
}
```

There is no `.nav-detail[hidden] { display: none; }` rule. Author CSS `display: flex` overrides the user-agent stylesheet's `[hidden] { display: none }`, so the empty detail div remains rendered. Combined with no `position: relative` on `.nav-sidebar`, the absolutely-positioned detail panel resolves against the viewport and covers everything.

## Acceptance criteria

- [ ] `.nav-detail[hidden] { display: none; }` honors the `hidden` attribute.
- [ ] `.nav-sidebar` is a positioned containing block so any `.nav-detail` stays inside the sidebar even if [hidden] toggles.
- [ ] Headless screenshot confirms NAV opens the sidebar without blanking the page.

## Resolution

### Reproduction

Headless test against `squad serve` confirmed the click on `#nav-toggle-btn` produced a fully blank screen (screenshot in `/tmp/browser-test/nav.png` pre-fix). DOM inspection showed `nav-detail` was `hidden=""` but Playwright reported it intercepts pointer events — the empty `.nav-detail` panel sat over the entire workspace.

### Fix

`internal/server/web/style.css`:
- Added `.nav-detail[hidden] { display: none; }` so the attribute actually hides the detail panel.
- Added `position: relative` to `.nav-sidebar` so any future `.nav-detail` is contained within the sidebar (not the viewport) even if [hidden] is toggled by error.

### Evidence

- `go test ./...` — all packages pass.
- Headless screenshot post-fix shows NAV opening the navigation sidebar with the board/chat/topbar all still reachable.
