---
id: CHORE-010
title: 'intake: move loadSession from commit_run.go to session.go'
type: chore
priority: P2
area: internal/intake
status: done
estimate: 15m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777294734
accepted_by: web
accepted_at: 1777303745
references: []
relates-to: []
blocked-by: []
---
## Refinement history
### Round 1 — 2026-04-27
this needs moe details

## Problem
`loadSession` — the helper that hydrates a `Session` value from the `intake_sessions` table — is defined at `internal/intake/commit_run.go:247` but is called from three sites that span the package: `commit_run.go:62` (item-only commit), `commit_spec_epic.go:30` (spec/epic commit), and `status.go:38` (read-only status RPC). Its placement is an artefact of git history — the session-loader was added when commit_run was its first consumer — not a reflection of where it belongs. Reading `status.go` today requires jumping into `commit_run.go` to chase a function whose body has nothing to do with committing.

## Context
`session.go` already owns the `Session` struct, the `OpenOrResume` lifecycle, the `ErrIntakeNotFound` sentinel, and the package-level docs about session shape. Moving `loadSession` next to those puts session-loading where a future maintainer will look first. Pure refactor: no behavior change, no signature change, no new tests.

The function reads exactly one row from `intake_sessions` via `db.QueryRowContext`, returns `ErrIntakeNotFound` on `sql.ErrNoRows`, and otherwise hydrates a `Session` with `time.Time` and `sql.Null{String,Int64}` decoding. It's roughly 30 lines and uses only standard imports (`context`, `database/sql`, `errors`, `time`) that `session.go` already pulls in.

## Acceptance criteria
- [ ] `loadSession` (and only that function) is moved verbatim from `internal/intake/commit_run.go` to `internal/intake/session.go`. All three call sites (`commit_run.go:62`, `commit_spec_epic.go:30`, `status.go:38`) continue to compile unchanged because the symbol stays package-scoped.
- [ ] `commit_run.go`'s import block is unchanged or shrinks (no import is added that wasn't there before; any import that was only used by `loadSession` moves with it). `session.go`'s import block adds at most one entry.
- [ ] `go test ./internal/intake/... ./cmd/squad/...` passes; no test files are edited.
- [ ] `go vet ./...` clean.

## Notes
- 15min estimate stands; this is a `git mv`-shaped change inside a single file boundary.
- Pure refactor — no PM-trace fingerprints, no behavior change, no new abstraction. Three similar callers stays three similar callers.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
