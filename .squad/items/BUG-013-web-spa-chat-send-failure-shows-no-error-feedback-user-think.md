---
id: BUG-013
title: web SPA — chat send failure shows no error feedback, user thinks message sent
type: bug
priority: P2
area: spa
status: open
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777241472
accepted_by: web
accepted_at: 1777241653
references: []
relates-to: []
blocked-by: []
---

## Problem

`internal/server/web/chat.js:119-131` (the compose-submit handler) wraps the POST in `try/catch` and on failure does only `console.warn('send failed:', err)`. The input is cleared and the user gets no UI signal. The user assumes the message went through — they only discover otherwise when a peer fails to respond, or when scrolling back through chat much later.

## Context

This is the only place a human user produces data through the dashboard. Silent send-failure here breaks the trust loop the SPA exists to support. The fix is small but the UX choice matters: don't clear the input until success is confirmed, surface an inline error or toast, and ideally give a "retry" affordance.

## Acceptance criteria

- [ ] On send failure, the compose input retains the unsent message so the user can retry without retyping.
- [ ] An inline error (or persistent toast) appears next to the compose box, announcing the failure with enough detail to action.
- [ ] On success, the input clears as today.
- [ ] No regression in the success path — Enter still posts, focus behavior unchanged.

## Notes

Found during a parallel exploration sweep on 2026-04-26. Surface change should be minimal — likely 10–20 lines in `chat.js` plus a small CSS rule.

## Resolution
(Filled in when status → done.)
