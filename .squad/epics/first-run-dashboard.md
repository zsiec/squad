---
spec: mcp-dashboard-bootstrap
status: open
parallelism: |
  Wave 1 (independent, dispatch in parallel): items #1 (GET /api/version), #2 (POST
  /api/_internal/restart), #3 (bootstrap package skeleton), #8 (symmetric uninstall),
  and #12 (Windows graceful skip). None of these touch shared symbols beyond their
  own files.

  Wave 2 (after wave 1, parallel): items #4 (probe + ensure-daemon logic), #6
  (welcome sentinel + auto-open), and #7 (first-run banner). #4 depends on #1, #2,
  #3; #6 and #7 depend on #3 (interfaces in bootstrap package).

  Wave 3 (after wave 2): item #5 (wire bootstrap into squad mcp boot). Single
  serialisation point — needs the building blocks complete.

  Wave 4 (after wave 3, parallel): items #9 (E2E bootstrap test), #10 (upgrade-flow
  integration test), #11 (README + adopting.md update). Tests assert observed
  behaviour; docs reflect shipped behaviour.

  Recommended dispatch: 1+2+3+8+12 → 4+6+7 → 5 → 9+10+11.
---

## Goal

Replace the manual `squad serve` step in the Quick Start with a zero-step first-run flow driven from the MCP server boot path, and make binary upgrades transparently restart the daemon on the new version. Detail in spec `mcp-dashboard-bootstrap` and design at `docs/plans/2026-04-27-mcp-driven-dashboard-bootstrap-design.md`.

## Child items

The 12 implementation tasks are filed as child items, each with `epic: first-run-dashboard` in their frontmatter. Acceptance criteria for each item are written so a fresh agent can execute the item without reading the spec.

## Anti-patterns to avoid during execution

- **No new user-facing concepts.** The user already knows about the plugin and the dashboard URL. Don't introduce new flags, env vars, or sentinels beyond `SQUAD_NO_AUTO_DAEMON` and `SQUAD_NO_BROWSER`.
- **No bypassing existing infra.** `internal/tui/daemon/` already implements `Install` / `Uninstall` / `Reinstall` / `Status` per platform. The bootstrap calls into it; it does not reimplement service files.
- **No blocking MCP serving on bootstrap failures.** Every failure path logs to stderr and continues. Squad without a UI is still useful.
- **No bundling waves into one commit.** Each item is one commit (or a small handful) so reverts are surgical.
- **No new schema, no migrations, no DB writes from bootstrap.** Bootstrap reads `os.Executable()`, hits a loopback HTTP endpoint, and calls `daemon.Manager` — that's the entire surface.
