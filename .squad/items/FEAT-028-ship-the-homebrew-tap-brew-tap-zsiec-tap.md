---
id: FEAT-028
title: ship the Homebrew tap (brew tap zsiec/tap)
type: feature
priority: P3
area: release
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777302756
accepted_by: web
accepted_at: 1777302856
references: []
relates-to: []
blocked-by: []
---

## Problem
The install path in `README.md:14` and `docs/adopting.md:9, :27` tells readers the Homebrew tap is "planned but not shipped" and points them at `go install github.com/zsiec/squad/cmd/squad@latest`. The intended user experience (described in `docs/contributing.md:108`) is `brew tap zsiec/tap && brew install squad`, with goreleaser auto-opening the formula PR on tag. Today nothing in CI or `.goreleaser.yaml` actually does that.

## Context
`.goreleaser.yaml:42` has a deferred TODO: `# Phase 14 will add: brews, release notes, signing`. The goreleaser config already builds binaries for `linux/{amd64,arm64}` and `darwin/{amd64,arm64}` (matching the release matrix in `CLAUDE.md`), so the tap formula has working artifacts to point at — what's missing is:

- The external `homebrew-tap` repo under the user's GitHub account (out-of-band; goreleaser pushes a formula PR but does not create the repo).
- A `brews:` block in `.goreleaser.yaml` referencing that tap.
- A CI secret with push rights on the tap repo so the cross-repo PR can land.
- Doc updates so the adoption surface stops telling readers the tap doesn't exist.

`docs/contributing.md:108` ("Homebrew tap PR is auto-opened") is the load-bearing description of the intended flow — implementation should match that contract.

## Acceptance criteria
- [ ] A `homebrew-tap` repository under the user's GitHub account exists with a `Formula/` directory. (Manual prerequisite — note this in the item.)
- [ ] `.goreleaser.yaml` gains a `brews:` block targeting that tap: formula name `squad`, license, homepage, description, and the cross-repo commit settings goreleaser v2 expects.
- [ ] The release CI workflow has access to a `HOMEBREW_TAP_GITHUB_TOKEN` (or equivalent) secret with push rights on the tap repo, and the goreleaser invocation passes it through.
- [ ] `README.md:14`, `docs/adopting.md:9` and `:27`, and the trailing comment in `.goreleaser.yaml` are updated: "planned but not shipped yet" wording removed, `brew tap zsiec/tap && brew install squad` documented as the primary install path, `go install` retained as an alternate.
- [ ] A test release (snapshot or pre-release tag) opens an auto-PR on the tap repo containing a working formula; installing that formula on a clean macOS box yields a runnable `squad` binary.

## Notes
- Type was inferred via `squad-capture` and the captured title was a doc fragment; refined title is the actionable form.
- The brews block must use `goarm` / multi-arch handling that matches the existing `goarch: [amd64, arm64]` build matrix so Apple Silicon users get an arm64 bottle.
- No user is currently blocked — `go install` works — so this is P3 adoption ergonomics, not urgent.
- This unblocks the `docs/contributing.md:108` claim being literally true; until then that line is aspirational.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
