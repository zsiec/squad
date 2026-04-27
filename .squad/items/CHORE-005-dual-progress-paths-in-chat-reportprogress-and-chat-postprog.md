---
id: CHORE-005
title: dual progress paths in chat.ReportProgress and chat.PostProgress
type: chore
priority: P3
area: chat
status: open
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777255647
accepted_by: web
accepted_at: 1777255754
references: []
relates-to: []
blocked-by: []
---

## Problem

`cmd/squad/progress.go` writes the same progress event twice on every call: `chat.ReportProgress` updates the `progress` table, and `chat.PostProgress` writes a row into the `messages` table. The inline comment near `cmd/squad/progress.go:59-65` acknowledges the duplication. `chat.LatestProgress` reads only from the `progress` table — divergence between the two writes silently surfaces only if a future caller reads from `messages` for progress.

## Context

The split exists because `progress` is a single-row-per-claim summary (used by hygiene, dashboards, latest-state queries) while `messages` is the chat ledger (used by tail/history/digest). Collapsing them is risky — they have different read patterns and TTLs. But two writes per progress call is brittle: a partial failure between them leaves the two tables disagreeing.

References:
- `cmd/squad/progress.go:47-65` (the dual-write site)
- `internal/chat/progress.go` (`ReportProgress`, `PostProgress`, `LatestProgress`)

## Acceptance criteria

- [ ] Either: (a) wrap the two writes in a single transaction so they succeed-or-fail atomically, OR (b) make one write the source of truth and derive the other (e.g., a trigger or read-side projection).
- [ ] Add a regression test that simulates a mid-flight failure between the two writes and asserts the two tables agree on the outcome.
- [ ] Update or remove the inline comment that acknowledges the duplication.

## Notes

P3, low risk — no user-visible bug today. File before it bites.

## Resolution

Chose option (a): merged the messages-row insert into ReportProgress's existing transaction. Now a single atomic write covers all four sites — `progress` row, `agents.last_tick_at`, `claims.last_touch`, and the `messages` row that feeds the SSE pump. Bus event still fires only after commit.

`PostProgress` is gone; the only caller (`cmd/squad/progress.go`) now makes a single `ReportProgress` call. The CHORE-005-acknowledged comment block on PostProgress is gone with it; the new ReportProgress doc names the four-site invariant.

New regression test `TestReportProgress_AtomicWithMessageInsert` plants a forced ABORT trigger on `messages` insert and asserts both `progress` and `messages` tables remain empty after the failure. New happy-path test `TestReportProgress_WritesBothProgressAndMessage` asserts a single call writes both.
