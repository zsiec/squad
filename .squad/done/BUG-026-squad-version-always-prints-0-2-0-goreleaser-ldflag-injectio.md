---
id: BUG-026
title: squad version always prints 0.2.0; goreleaser ldflag injection is no-op against const
type: bug
priority: P2
area: release
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777304720
accepted_by: web
accepted_at: 1777305671
references: []
relates-to: []
blocked-by: []
---

## Problem

`squad version` prints the hardcoded `0.2.0` regardless of the actual release tag. Verified after `brew install squad` from v0.3.0-rc2: the brew-installed binary at `/opt/homebrew/bin/squad` still prints `0.2.0` instead of `0.3.0-rc2`.

## Context

Root cause: `cmd/squad/main.go:19` declares `const versionString = "0.2.0"`. Go linker ldflags (`-X main.versionString=...`) can only override **vars**, not consts. The goreleaser ldflag injection at `.goreleaser.yaml` line 18 (`-X main.versionString={{.Version}}`) is silently a no-op for every release.

This affects every binary distribution:
- `brew install squad` (just shipped via FEAT-028) — wrong version
- `go install github.com/zsiec/squad/cmd/squad@vX.Y.Z` — wrong version
- Direct tarball downloads — wrong version

Complication: `cmd/squad/install_hooks_settings.go:14` has `const squadHookVersion = versionString`, which currently relies on `versionString` being a const. Changing `versionString` to `var` requires either making `squadHookVersion` a var too, or replacing the usage with a function/method.

The plan doc at `/Users/zsiec/dev/switchframe/docs/plans/2026-04-24-squad-phase-14-release.md` Task 13 anticipates this for `go install` only and recommends switching to `runtime/debug.ReadBuildInfo()` to read the version from module info (which works for both `go install` and ldflag-injected goreleaser builds).

## Acceptance criteria

- [ ] After `brew install squad` from a tagged release (e.g. v0.3.0-rc3 or later), `squad version` prints the tag version (with leading `v` stripped).
- [ ] After `go install github.com/zsiec/squad/cmd/squad@vX.Y.Z`, `squad version` prints `vX.Y.Z` (with leading `v` stripped).
- [ ] In a dev build (`go build` from source), `squad version` prints a sensible default (e.g., `0.0.0-dev` or the current `versionString` default, NOT empty).
- [ ] `cmd/squad/main_test.go` continues to pass without modification, or is updated to reflect the new version source.
- [ ] `install_hooks_settings.go` and any other downstream uses of `versionString` continue to compile and produce sensible output.

## Notes

Recommended approach (per plan doc Task 13): use `runtime/debug.ReadBuildInfo()` as the canonical source. Fall back to a default for `go test` / `go run` paths where build info isn't populated. This is one cohesive change that fixes both the `go install` and `brew install` paths in one shot.

Alternative if `ReadBuildInfo` is too invasive: just `const` → `var` on `versionString`, and adjust `squadHookVersion` accordingly. That fixes goreleaser builds but leaves `go install` printing the default.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
