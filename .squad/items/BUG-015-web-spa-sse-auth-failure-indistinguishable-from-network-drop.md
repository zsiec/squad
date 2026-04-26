---
id: BUG-015
title: web SPA — SSE auth failure indistinguishable from network drop, no re-auth prompt
type: bug
priority: P2
area: spa
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241479
accepted_by: web
accepted_at: 1777241684
references: []
relates-to: []
blocked-by: []
---

## Problem

`internal/server/web/app.js:200` handles SSE failure with `es.onerror = () => { setConn('disconnected'); };`. A 401 from the server (token expiry, daemon-side identity rotation) looks identical to a transient network drop. The user sees "disconnected", waits for it to come back, and never realizes they need to re-auth — the dashboard goes dark silently.

## Context

This matters more once the user is logged in for hours and an auth token rotates underneath them. The SSE channel is the dashboard's primary live-data path — if it dies because of auth and the UI does not say so, the user concludes "the daemon is broken" and restarts things they did not need to restart.

## Acceptance criteria

- [ ] On SSE error, distinguish auth failure (e.g., readyState transitions + a probe fetch to `/api/whoami` returning 401) from transient network failure.
- [ ] Auth-failure path shows a re-auth prompt (banner / modal) instead of the generic "disconnected" state.
- [ ] Network-drop path remains as-is.
- [ ] No regression on healthy reconnect after a brief network blip.

## Notes

Found during a parallel exploration sweep on 2026-04-26. Likely paired with the EventSource lifecycle work in BUG-014 since both touch `app.js`.

## Resolution

### Fix

`internal/server/web/app.js`:
- New `probeAuth()` runs a side-channel `fetch('/api/whoami')` on SSE error. 401/403 → `setConn('auth-failed')`. Network failure on the probe leaves state as `'disconnected'` (the original behavior), since EventSource will keep retrying.
- Reentry-guard `authProbing` prevents concurrent probes during retry storms; an additional gate skips the probe once `auth-failed` is latched, so a permanently-401'd EventSource doesn't poll whoami forever.
- `setConn` toggles a new `#auth-banner`. `onopen` resetting state back to `connected` clears the banner automatically.

`internal/server/web/index.html` — added `<div id="auth-banner" class="auth-banner" hidden role="alert">` above `<main>`.

`internal/server/web/style.css` — `.auth-banner` rule (red tint) + `.conn-dot[data-state="auth-failed"]` color.

### Reproduction / evidence

Playwright with two route-stub scenarios:

```
[auth-fail] state: {"conn":"auth-failed","bannerVisible":true}
[net-drop]  state: {"conn":"disconnected","bannerVisible":false}
```

Screenshots:
- `/tmp/browser-test/sse-auth-fail.png` — red banner + AUTH-FAILED label.
- `/tmp/browser-test/sse-net-drop.png` — DISCONNECTED label only, no banner.

### AC verification

- [x] Distinguish auth from network — whoami probe inspects status code.
- [x] Auth-failure path shows the banner.
- [x] Network-drop path remains as-is (disconnected, no banner).
- [x] Healthy reconnect: `onopen` flips state back to `connected`, banner auto-clears.
