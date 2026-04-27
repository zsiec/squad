---
id: CHORE-006
title: duplicate inboxEntry struct between server and tui client
type: chore
priority: P3
area: server
status: done
estimate: 30m
risk: low
evidence_required: []
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777255650
accepted_by: web
accepted_at: 1777255754
references: []
relates-to: []
blocked-by: []
---

## Problem

Two field-identical structs describe the same wire shape:
- `internal/server/inbox.go:9` — `inboxEntry` (lowercase, server-internal)
- `internal/tui/client/inbox.go:11` — `InboxEntry` (exported, client-facing)

Either side evolving in isolation produces silent decode mismatches. The split is defensible (server-internal vs client-API), but flagging here so it doesn't drift.

## Context

The server has serialized the shape into JSON; the tui client deserializes it. Today the field names match exactly. Future evolution paths:
- New optional field on the server: client deserialize works, but the client never surfaces it.
- Renamed field on the server: client deserialize silently zeroes it.
- Schema-tightened to required: client breaks.

References:
- `internal/server/inbox.go:9` (`inboxEntry`)
- `internal/tui/client/inbox.go:11` (`InboxEntry`)

## Acceptance criteria

- [ ] Choose: (a) extract the shared shape to a small internal/api package both sides import, (b) lock the contract with a JSON-schema test that loads a fixture and round-trips through both structs, OR (c) document the intentional split in both files' godoc and accept the drift risk.
- [ ] Whichever path: add a single test that fails if a field is added to one struct without the other.

## Notes

P3, low risk. Catches a class of regressions cheaply.

## Resolution
(Filled in when status → done.)
