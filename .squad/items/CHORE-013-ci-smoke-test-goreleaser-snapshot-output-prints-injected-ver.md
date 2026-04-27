---
id: CHORE-013
title: 'CI: smoke-test goreleaser snapshot output prints injected version'
type: chore
priority: P2
area: release
status: open
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-401f
captured_at: 1777306242
accepted_by: agent-bbf6
accepted_at: 1777306827
references: []
relates-to: []
blocked-by: []
---

## Problem
BUG-026 just fixed the regression where goreleaser's `-X main.versionString={{.Version}}` ldflag (`.goreleaser.yaml:18`) silently stopped landing — `versionString` was a const, not a var, so the snapshot binary kept printing `0.2.0` instead of the injected snapshot version. The fix is in, but nothing in CI proves the next refactor of `cmd/squad/main.go:20-33` will not regress the same way. CI today runs `go test`, `go vet`, `go build`, and a cross-arch `go build`, but never executes a goreleaser-built binary.

## Context
Version-printing path is: `cmd/squad/main.go:20` declares `var versionString string`; `init()` (`:22-33`) falls back to `debug.ReadBuildInfo()`, then to `"0.0.0-dev"`; the `version` subcommand (`:177-186`) prints the value via `fmt.Fprintln(cmd.OutOrStdout(), versionString)`. The ldflag in `.goreleaser.yaml:18` is the only path that injects a real version into a release artifact, and the snapshot template in `.goreleaser.yaml:32` produces values shaped like `<incpatch-version>-snapshot`.

CI lives in `.github/workflows/ci.yml`. There are three jobs: `test` (matrix ubuntu/macos × Go 1.25), `cross-build` (linux/darwin × amd64/arm64 plain `go build`), and `lint` (golangci-lint). None of them exercise goreleaser. Adding a fourth job that invokes goreleaser snapshot and runs the resulting binary is the smallest change that defends the BUG-026 fix.

## Acceptance criteria
- [ ] `.github/workflows/ci.yml` gains a new job (e.g. `release-smoke`) on `ubuntu-latest` that runs `goreleaser build --snapshot --single-target --clean` (or equivalent v2 invocation). Single-target keeps wall-clock low.
- [ ] After the build, the job executes the produced binary's `version` subcommand and asserts in shell:
  - The output is NOT the literal `0.0.0-dev` (the init() fallback that means no ldflag landed).
  - The output matches the snapshot version-template shape (`X.Y.Z-snapshot`, e.g. via `grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+-snapshot$'`).
- [ ] The job fails red if the ldflag stops landing — i.e. running it against the BUG-026 pre-fix code (where `versionString` was a const) would have failed this assertion.
- [ ] No new Go dependencies; the smoke is shell-only and uses `goreleaser/goreleaser-action@vN install-only:true` (or `apt`/`brew` install) to obtain the binary.
- [ ] Existing `test`, `cross-build`, and `lint` jobs unaffected. Total wall-clock added: under a minute on cold cache.

## Notes
- `--single-target` builds only the current host's GOOS/GOARCH (linux/amd64 on `ubuntu-latest`), which is sufficient to validate ldflag injection — the ldflag applies identically across the matrix per `.goreleaser.yaml:14-18`.
- The smoke is a guard against the BUG-026 regression class, not a release gate; it does not need to publish artifacts or sign anything. Skip release-publish steps explicitly if goreleaser tries to invoke them.
- Follow-up worth filing if this lands cleanly: extend the smoke to assert the same property on the four-arch matrix, but only if/when CI cost stays acceptable. Out of scope here.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
