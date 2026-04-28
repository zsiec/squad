---
id: FEAT-059
title: squad retro generates weekly digest of failure modes and process suggestions
type: feature
priority: P2
area: stats
status: done
estimate: 1d
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777345159
accepted_by: web
accepted_at: 1777345480
references: []
relates-to: []
blocked-by: []
epic: team-practices-as-mechanical-visibility
---

## Problem

The SQLite ledger captures every claim, release, blocker, attestation,
chat post, and item state transition, but nothing synthesizes the
week's signal into a durable artifact. Recurring pain points stay
per-incident — the operator can't easily see "we kept hitting X-class
problems this week" without ad-hoc queries. Human teams use a weekly
retro for exactly this; the agent-realm equivalent is a generated
markdown file the next session's agents can auto-load.

## Context

`internal/stats/` already aggregates ledger metrics for the dashboard.
`internal/learning/` has the auto-load convention: approved entries
synthesize into `.claude/skills/squad-learnings.md` and are picked up
by skill-paths globs. The retro just needs to write to a parallel
location with the same auto-load contract.

This is read-only on the ledger — no mutations to claims/items/chat —
so it's safe to run on a cron without coordination.

## Acceptance criteria

- [ ] `squad retro` reads the last 7 days from the global SQLite
      ledger and writes `.squad/retros/YYYY-WW.md` (ISO week). The
      file contains, at minimum:
      - Top 3 failure modes (e.g. items closed as `blocked`,
        attestation kinds with high failure rate, repeated
        no-repro reclassifications).
      - Slowest item type to close (median time-from-claim-to-done
        per `type:`).
      - One process recommendation derived from the data (e.g.
        "specs-area items took 4x median this week — consider
        decomposition before claim").
- [ ] The output is deterministic for fixed input — same DB state
      produces the same file. Useful for testing.
- [ ] Skill paths glob loads the most recent retro automatically when
      an agent starts a session in the repo, same convention as
      `squad-learnings.md`.
- [ ] `squad retro --week YYYY-WW` regenerates a specific week
      (idempotent overwrite).
- [ ] Test: seed a fixture DB with known item/claim/attestation
      shape, run `squad retro`, assert the markdown contains each
      of the three sections and the expected statistics.
- [ ] No-op when the ledger holds <N items in the period (configurable
      threshold; default 5). The output instead writes a one-line
      "insufficient signal" file so cron runs don't pile up empty
      retros.

## Notes

- The recommendation line is the hard part. Keep it rule-based for
  this item — a small set of pattern matchers ("if median(blocked)
  > X" → recommend Y). LLM-generated recommendations are a separate
  item if the rule set proves insufficient.
- Lands second in the epic sequence: FEAT-057 makes peer state
  passive in real time; FEAT-059 makes weekly state durable. The
  recommendations the retro emits will inform whether FEAT-058,
  area auto-mention, and auto-postmortem ship in their proposed
  forms.

## Resolution
(Filled in when status → done.)
