---
id: BUG-035
title: AGENTS.md Recently done section omits the close summary required by FEAT-049 AC
type: bug
priority: P2
area: internal/scaffold
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777323735
accepted_by: web
accepted_at: 1777325379
references: []
relates-to: []
blocked-by: []
---

## Problem

FEAT-049 AC explicitly required the "Recently done" section to render
`(id, title, summary)` per item. The implementation in
`internal/scaffold/agents_md.go:71-75` renders `**ID** (priority) — title`
— priority not summary, and no summary anywhere on the line. The
`Item` struct has no `Summary` field; the close summary is recorded
only as a chat message body (`done <id>: <summary>`) in
`internal/claims/done.go:46-49`, and the cobra wrapper
`scaffold_agents_md.go` never queries that table.

## Context

The close summary is the single most useful piece of context per done
item — without it the section just lists IDs and titles already
visible elsewhere. AC source (`.squad/done/FEAT-049-...md`):

```
- [ ] Output sections include: top 5 ready items (id, title, priority);
  in-flight claims (id, title, claimant, intent); last 10 done items
  (id, title, summary); active specs and epics index ...
```

Current rendered output (`go run ./cmd/squad scaffold agents-md`):

```
## Recently done

- **BUG-017** (P2) — session_start hook leaves agents in repo_id=_unscoped — invisible to squad who
- **BUG-018** (P3) — inbox modal rows go stale on inbox_changed SSE — only the badge updates
...
```

— priority where AC asked for summary, no summary at all.

## Acceptance criteria

- [ ] `RenderAgentsMd` emits each "Recently done" line with the close
      summary recorded at done time (e.g. via a per-item lookup keyed
      on `item_id` against the `messages` table for `kind='done'`
      messages, or via a new field on `items.Item` populated from that
      query — design choice deferred to implementer).
- [ ] When an item was closed without a summary the line falls back to
      a clearly-marked placeholder rather than dropping the item.
- [ ] `internal/scaffold/agents_md_test.go` gains a fixture entry that
      pins the `(id, title, summary)` shape and a render assertion
      that the summary text appears in the output.
- [ ] Running `squad scaffold agents-md` against the live ledger
      produces a "Recently done" section whose lines include the
      summaries actually used at `squad done --summary` time.

## Notes

Two viable shapes:

1. Pull summaries lazily in the cobra wrapper: query `messages` for
   `kind='done'` rows, build `map[itemID]string`, pass alongside
   `Done` items into `AgentsMdData`. Keeps `RenderAgentsMd` pure.
2. Add `Summary` to `items.Item` and have items.Walk populate it from
   a one-shot SELECT. Wider blast radius, more invasive.

Prefer (1).
