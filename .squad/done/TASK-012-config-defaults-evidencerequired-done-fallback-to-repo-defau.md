---
id: TASK-012
title: config Defaults.EvidenceRequired + Done() fallback to repo default
type: task
priority: P1
area: cli
status: done
estimate: 1h
risk: medium
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777245990
accepted_by: agent-bbf6
accepted_at: 1777245990
epic: feature-uptake-nudges
evidence_required: [test]
references:
  - internal/config/config.go
  - cmd/squad/done.go
relates-to: []
blocked-by: []
---

## Problem

`Done()` already enforces `evidence_required` per item — but reads the field only from the per-item frontmatter (`cmd/squad/done.go:86`). Items that omit the field skip the gate, so the existing attestation enforcement never fires repo-wide. Dogfood data over the first 25 closed items: zero attestations recorded.

## Context

`internal/config/config.go:63-68` defines a `Defaults` struct already (priority/estimate/risk/area). Adding a fifth field — `EvidenceRequired []string` — and consulting it in `Done()` when the item is silent is the highest-leverage way to light up the existing gate, since operators can opt their whole repo in via one config line.

## Acceptance criteria

- [ ] `internal/config/config.go` `Defaults` struct gains `EvidenceRequired []string` with yaml tag `evidence_required`.
- [ ] Unit test: `defaults:\n  evidence_required: [test, review]\n` in `.squad/config.yaml` round-trips through `Load()` to `cfg.Defaults.EvidenceRequired == []string{"test", "review"}`.
- [ ] `cmd/squad/done.go` `DoneArgs` gains `DefaultEvidenceRequired []string` field.
- [ ] `Done()` falls back to `args.DefaultEvidenceRequired` when `parsed.EvidenceRequired` is empty; non-empty per-item field still wins.
- [ ] The cobra wrapper in `done.go` populates `DefaultEvidenceRequired` from `config.Load(repoRoot).Defaults.EvidenceRequired`.
- [ ] Every other in-process `Done(...)` caller (search `grep -rn "Done(ctx, DoneArgs"`) is updated to either populate the field or explicitly pass `nil` (preserving current behavior).
- [ ] Integration-style test in `cmd/squad/done_test.go`: an item without per-item `evidence_required`, plus a config that sets `defaults.evidence_required: [test]`, returns `*EvidenceMissingError` with `Missing == [KindTest]` from `Done()`.
- [ ] `go test ./...` passes; trailing `ok` line pasted into the close-out chat.

## Notes

- No new schema, no migration. The `attestations` table and `Done()` gating logic are unchanged.
- `Load()` returns the zero-value `Defaults{}` when `.squad/config.yaml` is absent — that means `EvidenceRequired` is `nil` and behavior matches today.
- This item unlocks TASK-014 (scaffold writes the default into newly-init'd repos).

## Resolution

### Fix

`internal/config/config.go` — `Defaults` gains `EvidenceRequired []string` (`yaml:"evidence_required"`). Zero-value is `nil`, matching prior behavior when the field is absent.

`cmd/squad/done.go`:
- `DoneArgs` gains `DefaultEvidenceRequired []string`.
- `Done()` checks `parsed.EvidenceRequired` first; if empty, falls back to `args.DefaultEvidenceRequired`. Replace-not-merge — a non-empty per-item list always wins outright.
- The cobra wrapper hoists `cfg, _ := config.Load(repoRoot)` above the verify gate (used to be scoped inside `if !skipVerify`) so the same cfg drives `cfg.Verification.PreCommit` and populates `DefaultEvidenceRequired`. Behavior under `--skip-verify` is unchanged: `Load` returns `Config{}` on error, so the field is `nil` and `Done()` matches prior behavior.

`cmd/squad/mcp_register.go` — same `Done` call site updated; added `internal/config` import.

`grep -rn "Done(.*DoneArgs"` confirms the only production call sites are these two; both populate the new field. The five test-file call sites pass nil-equivalent zero-values, preserving prior behavior.

### Tests

- `internal/config/config_test.go::TestLoad_DefaultsEvidenceRequired` — `defaults: { evidence_required: [test, review] }` round-trips.
- `cmd/squad/done_lib_test.go::TestDone_PureFallsBackToDefaultEvidenceRequired` — item silent on `evidence_required`; default `[test]` triggers `EvidenceMissingError` with `Missing == [KindTest]`.
- `cmd/squad/done_lib_test.go::TestDone_RoundTripsConfigDefaultsThroughDone` — full chain: write `.squad/config.yaml`, `config.Load`, into `Done()`, assert the gate fires.
- `cmd/squad/done_lib_test.go::TestDone_PerItemEvidenceWinsOverDefault` — per-item `[test, review]` beats default `[manual]`; "manual" never appears in `Missing`.

### Evidence

```
$ go test ./... -count=1 -race
... ok  github.com/zsiec/squad/templates/github-actions  4.606s
(0 FAIL lines)
```

### AC verification

- [x] `Defaults.EvidenceRequired` field present with right yaml tag.
- [x] Round-trip test passes.
- [x] `DoneArgs.DefaultEvidenceRequired` field added.
- [x] Replace-not-merge fallback semantic implemented.
- [x] Cobra wrapper populates from `config.Load`.
- [x] All in-process `Done(...)` callers updated (production sites populate; tests pass nil to keep current behavior).
- [x] Integration-style test in `done_lib_test.go` covers the full config → Done chain.
- [x] `go test ./... -count=1 -race` passes.
