---
id: CHORE-021
title: standup digest surfaces unread mentions before last touch ordering
type: chore
priority: P3
area: cmd
status: open
estimate: 1h
risk: low
evidence_required: []
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352051
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
---

## Problem

The standup digest at `squad go` (FEAT-057) sorts active peer
claims by `last_touch DESC`. When the mailbox carries an unread
`@<me>` mention, the digest doesn't surface that — the agent has
to read the digest, then run `squad inbox` (or scroll the mention
flush) to learn who's blocking on them. Two-step where one would
do.

## Context

The digest builder in `cmd/squad/go.go` (or wherever FEAT-057 put
it) reads `claims` + `messages` to assemble the `peers:` block.
Add a pre-pass: query `messages WHERE mentions LIKE
'%@<me>%' AND ts > <last_read_ts>` and prepend the matching peers
above the last-touch ordering, with a small visual marker (`*` or
similar prefix) indicating "this peer is asking for you".

## Acceptance criteria

- [ ] When the mailbox has unread mentions of the current agent,
      the `peers:` block at `squad go` lists those peers first,
      with a marker that disambiguates them from regular last-
      touch entries.
- [ ] Peers without unread mentions still show in the original
      last-touch order below.
- [ ] Cap and `+N more` truncation behavior is preserved
      (mention-prioritized peers count toward the cap; the
      overflow first drops non-mention peers).
- [ ] Test: seed a fixture ledger with two active peers (one
      with an unread mention, one without) and assert ordering.
- [ ] Test: seven peers, three with mentions; verify the three
      land at the top and the cap-of-six rule still holds.

## Notes

- Small, additive change to the existing digest. No new tables
  or columns.
- Pairs naturally with the cross-session async review pattern
  the team's been using — making mentions impossible to miss at
  session start tightens the loop.

## Resolution
(Filled in when status → done.)
