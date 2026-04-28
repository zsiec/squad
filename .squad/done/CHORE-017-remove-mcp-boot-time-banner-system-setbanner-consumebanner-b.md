---
id: CHORE-017
title: remove MCP boot-time banner system (SetBanner / ConsumeBanner / BannerInstalled-Upgraded-PortConflict-Unsupported)
type: chore
priority: P2
area: internal/mcp/bootstrap
status: done
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-afcd
captured_at: 1777339943
accepted_by: agent-afcd
accepted_at: 1777339977
references: []
relates-to: []
blocked-by: []
---

## Problem

The MCP boot-time banner system (`internal/mcp/bootstrap/banner.go`)
stages a one-shot string for the next `tools/call` response — used to
surface "Squad dashboard ready at ..." after a fresh daemon install,
"Squad upgraded to ..." after a version skew, "port in use", and the
unsupported-platform notice. Per direct user feedback the mechanism
is not delivering on its intent — agents do not surface the banner
to the human in a useful way and the staging cost (atomic Value,
ConsumeBanner branch in every successful tool call, banner-copy
tests) outweighs the value.

## Context

Surface to remove:

- `internal/mcp/bootstrap/banner.go` — `SetBanner`, `ConsumeBanner`,
  `BannerInstalled(port)`, `BannerUpgraded(version)`,
  `BannerPortConflict(port)`, `BannerUnsupported`.
- `internal/mcp/bootstrap/ensure.go` — the `SetBanner(...)` calls in
  the install / upgrade / port-conflict / unsupported branches.
- `internal/mcp/server.go` — the `Server.SetBanner` facade and the
  `ConsumeBanner`-prepend block in `callTool`.
- Tests that exist solely to pin banner behavior:
  `internal/mcp/server_banner_test.go`,
  `internal/mcp/bootstrap/banner_copy_test.go`,
  banner cases inside `bootstrap_test.go`, `ensure_test.go`,
  `ensure_unsupported_test.go`, `upgrade_test.go`,
  `cmd/squad/mcp_test.go`, `cmd/squad/mcp_bootstrap_integration_test.go`.

The dashboard URL stays surfaced via stdout in `squad serve`
(`cmd/squad/serve.go:195` — `Squad dashboard: http://...`). The
human-facing copy is unaffected; the MCP-channel banner is what
goes away.

## Acceptance criteria

- [ ] `internal/mcp/bootstrap/banner.go` deleted.
- [ ] No remaining references to `SetBanner`, `ConsumeBanner`,
      `BannerInstalled`, `BannerUpgraded`, `BannerPortConflict`,
      or `BannerUnsupported` anywhere in the tree (verify with
      grep).
- [ ] `Server.SetBanner` and the `ConsumeBanner`-prepend block in
      `callTool` removed from `internal/mcp/server.go`.
- [ ] `Ensure` no longer stages banners; the install / upgrade /
      port-conflict / unsupported branches retain their existing
      stderr / log behavior but do not call `SetBanner`.
- [ ] All banner-only tests deleted; tests that exercise other
      behavior keep their non-banner assertions.
- [ ] `go test ./...`, `go vet ./...`, `golangci-lint run` all
      green.

## Notes

The dashboard URL still surfaces to the user via `squad serve`'s
stdout boot line. The banner channel was an MCP-side surfacing
attempt that didn't pan out; removing it does not regress the
human-facing UX.
