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

- [x] When the mailbox has unread mentions of the current agent,
      the `peers:` block at `squad go` lists those peers first,
      with a marker that disambiguates them from regular last-
      touch entries.
- [x] Peers without unread mentions still show in the original
      last-touch order below.
- [x] Cap and `+N more` truncation behavior is preserved
      (mention-prioritized peers count toward the cap; the
      overflow first drops non-mention peers).
- [x] Test: seed a fixture ledger with two active peers (one
      with an unread mention, one without) and assert ordering.
- [x] Test: seven peers, three with mentions; verify the three
      land at the top and the cap-of-six rule still holds.

## Notes

- Small, additive change to the existing digest. No new tables
  or columns.
- Pairs naturally with the cross-session async review pattern
  the team's been using — making mentions impossible to miss at
  session start tightens the loop.

## Resolution

`cmd/squad/peer_digest.go`:

- `peerRow` carries a new `HasMention bool`.
- `annotateMentions` runs one EXISTS query per peer
  (`messages JOIN reads`) checking for unread `body LIKE
  '%@<myAgentID>%'` rows in the peer's thread, mirroring the
  body-substring detection used in `internal/chat/digest.go`.
  Self-posts are excluded (`m.agent_id != ?`) so the caller's
  own `@me` reminders don't trigger.
- `sortByMentionThenLastTouch` does a stable partition: mentioned
  peers first, then non-mention; within each group the
  `last_touch DESC` ordering from the loader is preserved.
- `renderPeerDigest` adds a single `*` (or space) marker prefix
  per row. Both formats are 4 chars before `@`, so columns align.

Mention prioritization counts toward the cap-of-six. Overflow
drops non-mention rows first.

Tests:

- `TestPeerDigest_MentionedPeerSurfacesFirstWithMarker`: two-peer
  fixture where the mentioned peer was touched 9m ago and the
  non-mention peer 1m ago; mention surfaces first with `*`.
- `TestPeerDigest_SevenPeersThreeMentionsLandAtTop`: 7 peers with
  the 3 oldest carrying mentions; mention prioritization pulls
  them above the cap, `+1 more` collapses one of the non-mention
  rows.
- `TestPeerDigest_ReadMentionDoesNotPrioritize`: a mention past
  the agent's `reads.last_msg_id` does not prioritize. The
  unread bound is load-bearing — without it every old `@me`
  resurfaces forever.
- Pre-existing `TestPeerDigest_SevenPeersTruncatedToSixPlusOne`
  was counting rows via `\n  @` substring; updated to count via
  line iteration since the row prefix changed.

Coordination: @agent-afcd was concurrently editing
`cmd/squad/peer_digest.go` for CHORE-020 (removing `answer`
from `humanVerbKinds`); we acked via `squad ask` chain — diffs
touch different lines, fold cleanly.

Verification:

- `go test ./... -race -count=1` — every package `ok`.
- `golangci-lint run` — `0 issues.`

Code review: 0 blocking findings; one Low note about substring
agent-id collision (e.g. `@agent-meta` triggering for
`agent-me`) accepted for parity with the existing chat-digest
detector. Worth a follow-up CHORE if mention-detection is ever
hardened ledger-wide.
