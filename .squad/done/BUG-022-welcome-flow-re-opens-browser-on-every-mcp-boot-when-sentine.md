---
id: BUG-022
title: welcome flow re-opens browser on every MCP boot when sentinel write silently fails
type: bug
priority: P1
area: internal/mcp/bootstrap
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777293733
accepted_by: web
accepted_at: 1777293926
references: []
relates-to: []
blocked-by: []
---

## Problem
On every MCP boot the welcome flow opens the dashboard in the user's default browser, even after they've already seen it. The intended once-per-machine behavior is gated by `~/.squad/.welcomed`; in practice the sentinel is missing and Welcome re-fires repeatedly. Reproduced on the maintainer's macOS box: `~/.squad/.welcomed` does not exist despite many sessions, so Chrome auto-opens "all the time."

## Context
`internal/mcp/bootstrap/welcome.go` — Welcome opens the URL FIRST (line 30) and then writes the sentinel (line 35). Both steps log a stderr warning on failure and return nil — but stderr from `squad mcp` goes to Claude Code's MCP transport, not the user's terminal, so a sentinel-write failure is invisible. Caller is `cmd/squad/mcp.go:73` (`realBootstrap`) which runs synchronously on every MCP server boot.

Why the sentinel goes missing is unconfirmed — candidates: write actually fails silently (permissions, sandbox, t.TempDir-style isolation), or the install/uninstall cycle removes it via `cmd/squad/install_plugin.go:193` more often than we realize. The fix should be robust regardless of which it is.

## Acceptance criteria
- [ ] Welcome writes the sentinel BEFORE invoking the opener. Browser opens at most once per machine even if the opener crashes or the user kills the process between the open and the would-be sentinel write.
- [ ] Sentinel write uses an atomic temp+rename so a partial filesystem failure cannot leave a torn file that gets misread.
- [ ] If the sentinel write fails, Welcome returns the error (not nil). `realBootstrap` logs it to stderr as today; the change is that callers can detect the failure in tests and a future surface (banner / log) can react.
- [ ] Add a unit test covering: (a) opener-fails-but-sentinel-still-written; (b) sentinel-write-fails surfaces an error; (c) second Welcome call with sentinel present is a no-op (no opener invocation).
- [ ] Manual smoke on macOS: `rm ~/.squad/.welcomed && squad mcp <<<''` (or restart Claude Code session); Chrome opens once, sentinel is present, second invocation is silent.

## Notes
Order-of-operations fix is the primary one. Atomic write is belt-and-suspenders for the case where the user's home is on a flaky volume / sandboxed filesystem. The "return error not nil" tweak is what unblocks future visibility work without changing today's "MCP keeps serving even if welcome breaks" behavior — `realBootstrap` already logs and ignores.

Out of scope: deciding whether `install-plugin --uninstall` should keep removing `.welcomed` (that is current FEAT-013 design intent). If the user has been uninstalling, the sentinel will keep going away — but the fix here at least guarantees one open per install, not one per session.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
