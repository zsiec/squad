---
id: BUG-007
title: 'markdown.js renders javascript: URLs as live links — XSS via item body / chat'
type: bug
priority: P1
area: web-ui
status: open
estimate: 1h
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-1f3f
captured_at: 1777240191
accepted_by: web
accepted_at: 1777241665
references: []
relates-to: []
blocked-by: []
---

## Problem

`internal/server/web/markdown.js` runs `escapeHtml` on the input, then applies a `LINK = /\[([^\]]+)\]\(([^)]+)\)/g` regex to the escaped string. `escapeHtml` does not escape `[`, `]`, `(`, `)`, `:`, so a markdown link like `[click](javascript:alert(1))` survives and gets rendered as `<a href="javascript:alert(1)" target="_blank" rel="noopener">click</a>`. Clicking executes script in the dashboard origin.

## Context

Item bodies and chat messages flow through this renderer (`drawer.js` for board items, the new inbox-detail panel from BUG-006, and chat). Threat model is local-only — bodies are written by agents on the user's machine — but a hostile/compromised peer agent or a copy-pasted payload from anywhere can plant the link. `target="_blank" rel="noopener"` does nothing against `javascript:` schemes.

Pre-existing; surfaced during BUG-006 review. Not introduced by BUG-006.

## Acceptance criteria

- [ ] `[x](javascript:alert(1))` rendered through `renderMarkdown` does not produce an `<a>` whose `href` starts with `javascript:` (or `data:`, `vbscript:`).
- [ ] Allowed schemes for the emitted `href`: `http://`, `https://`, `mailto:`, and relative URLs (no scheme).
- [ ] Disallowed-scheme links render as plain text (still escaped) so the user sees the URL but cannot click it into script.
- [ ] Unit/regression test in `internal/server/web/` or equivalent — minimum a Go test that loads the JS into a JS runtime, OR an explicit string-comparison test in markdown.js itself if the project chooses to add a JS test harness.
- [ ] No regression in normal links (`[ok](https://example.com)`) or autolinked file refs.

## Notes

Smallest fix: validate the captured URL in `inline()` before splicing it into `href`; reject schemes outside an allowlist; for rejected schemes emit `<code>` or escaped text.

Surfaces consuming `renderMarkdown`: `drawer.js` (item body sections), `inbox.js` (BUG-006 inline detail panel), `chat.js`, `activity.js` (anywhere `.md` rendering is invoked).

Related: BUG-006 (inbox-detail panel that newly exposes this surface).

## Resolution

### Reproduction
`TestMarkdownLinkXSS` (Go test that drives `markdown.js` through Node) covers `javascript:`, `data:`, `vbscript:`, case-insensitive `JaVaScRiPt:`, leading-whitespace ` javascript:`, tab-in-scheme, and protocol-relative `//evil.com`. Pre-fix: 5 sub-tests failed with live `<a href="...">`. Post-fix: all 12 pass.

### Fix
`internal/server/web/markdown.js`:
- Added `isSafeURL(u)` helper. URL is safe if (after trim) it has no colon, OR a path/hash/query delimiter precedes the first colon, OR its scheme matches `^(?:https?:|mailto:)` (case-insensitive). Protocol-relative `//host` is rejected (it inherits the page scheme).
- The `LINK` replace callback now branches on `isSafeURL`: safe URLs render as `<a href="...">`; unsafe URLs round-trip as the literal escaped `[text](url)` text so the user sees the URL but cannot click it into script.

### Test
`internal/server/markdown_xss_test.go` — Go test that spawns `node --input-type=module --eval=<harness>` from `internal/server/web/`, stubs the browser globals util.js touches at module load (`location`, `localStorage`), imports `markdown.js`, runs `renderMarkdown` over a 12-case table, and asserts on the produced HTML. Skipped if `node` is not on PATH.

### Evidence
```
ok  	github.com/zsiec/squad/internal/server	0.443s   (TestMarkdownLinkXSS — 12/12)
```
Verified RED on prior code (5 failures), GREEN on fix.
