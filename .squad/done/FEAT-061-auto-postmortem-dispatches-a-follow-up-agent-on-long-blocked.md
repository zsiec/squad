---
id: FEAT-061
title: auto-postmortem fires when a claim ends without durable learning artifacts
type: feature
priority: P3
area: learning
status: done
estimate: 4h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777345159
accepted_by: web
accepted_at: 1777345485
references: []
relates-to: []
blocked-by: []
epic: team-practices-as-mechanical-visibility
---

## Problem

Some claims end without `outcome=done` *and* without any durable
record of what was tried or ruled out. The lesson dies in the chat
thread: a peer hitting the same wall later has to rediscover the
dead-ends. A human team would write a postmortem; the agent-realm
equivalent should capture the lesson before the context evaporates —
but only when the lesson hasn't already been captured by other means
(item-file edits, learning artifacts, substantive chat).

## Context

`internal/learning/` already has the artifact format
(`.squad/learnings/<slug>.md`, gotcha / pattern / dead-end kinds) and
the propose → human approve → auto-load flow. The skill
`squad-handoff` already prescribes a "surprised by" bullet on session
pause. What's missing is the trigger plus a detection check that
distinguishes "agent gave up with nothing learned" from "agent ran a
careful premise audit, posted detailed fyis, and cleanly released
because the work shouldn't ship in current form."

The original framing of this item gated on claim duration (>2h).
That misshapes the trigger: a 10-minute premise audit can produce
strong durable signal (see this item's own Premise audit section,
filed in <15min), and a 5-hour rabbit-hole might leave nothing. The
right detection criterion is *artifact presence*, not duration.

## Acceptance criteria

- [ ] When `squad release <ID> --outcome released` (or any non-`done`
      outcome) fires, run the artifact-presence detector. The same
      check runs when an item flips to `status: blocked` via the
      item-file path. The trigger is the close-out event, not a
      duration threshold.
- [ ] Detector returns "skip dispatch" when ANY of the following
      exist in the claim window (`claimed_at` → `released_at`).
      Authorship is intentionally ignored: the question is whether
      the durable record exists, not who wrote it. A peer's analysis
      captures the lesson just as well as the claimant's would.
      a) any new `## Premise audit`, `## Session log`,
         `## Blocker`, or `## Resolution` section appended to the
         item file (or any non-trivial body addition under an
         existing `## Notes`);
      b) any learning artifact created via `squad learning propose`
         (regardless of approval state);
      c) ≥`postmortem.min_chat_messages` chat posts on the item
         thread with body length ≥ `postmortem.min_chat_chars`
         (defaults: 2 messages, 50 chars).
      d) a previously-filed postmortem artifact named
         `<itemID>-postmortem-*.md` already exists under
         `.squad/learnings/` — reruns are idempotent.
- [ ] When the detector returns "dispatch", the system spawns a
      follow-up subagent under a `superpowers:postmortem` role with
      three inputs: the item file path, the full chat thread for the
      item, and the git diff (if any) of the claim window. The agent
      writes a `dead-end` learning artifact in the propose state —
      auto-slugged as `<itemID>-postmortem-<YYYYMMDD-HHMMSS>` (item
      ID over title for stability across renames) — following the
      standard structure (hypotheses tried, ruled-out causes,
      evidence collected, what to do differently).
- [ ] Configurable in `.squad/config.yaml` under `postmortem:`:
      `enabled: bool` (default true), `min_chat_messages: int`
      (default 2), `min_chat_chars: int` (default 50).
      `enabled: false` short-circuits the detector to "skip".
- [ ] Test: simulate a release with NO item-file delta, NO learning
      artifact, NO chat posts → detector returns "dispatch" and the
      postmortem agent invocation produces a propose-state learning
      file under `.squad/learnings/proposed/`.
- [ ] Test: simulate a release where the claimant added a
      `## Premise audit` section → detector returns "skip"; no
      learning artifact produced.
- [ ] Test: simulate a release where the claimant posted 3 fyi
      messages each ≥80 chars on the item thread → detector returns
      "skip".
- [ ] Test: `enabled: false` in config short-circuits the detector
      regardless of artifact state.

## Notes

- The detection criterion is intentionally additive: any one signal
  of durable learning suppresses the dispatch. This errs on the
  side of fewer dispatches (cheaper) and trusts that operators who
  bothered to leave artifacts have captured the lesson well enough.
- The postmortem agent's prompt must avoid blame language — the
  artifact is a lesson, not a verdict. The skill prompt should
  read: "describe what was tried, what was learned, what evidence
  ruled out which hypotheses; do not name agents in failure
  context."
- The dispatch mechanism is left to implementation discretion: a
  `squad release` post-action hook, a daemon listener on chat
  events, or a `squad postmortem <ID>` verb the operator runs
  manually with auto-detection. The AC pins WHEN to fire and WHAT
  to produce, not the exact wiring.
- This reshape (2026-04-28) replaced a duration-based trigger
  (>2h) that fired zero times in 7d (and all-time) of ledger data.
  Keep the detector logic empirically grounded — if a year of
  operation shows the dispatch never fires, the item is no longer
  pulling its weight regardless of how the AC reads.

## Premise audit (2026-04-28)

A claim attempt at 2026-04-28 23:51 ran a SQLite query against the
chat ledger before doing any work. Last 7 days AND all-time:

- `blocked` chat-verb events: **0**
- `release --outcome released` events with hold time > 2h: **0**
- `stuck` posts: 5 (none escalated to blocked)

The original AC's duration triggers (`squad blocked` >2h,
`release --outcome released` >2h) fired zero times in any window.
Reshape (this commit): drop the duration gate; switch detection to
*artifact presence*. With that change, the new retro signal of "~6
releases-without-done/week" becomes the candidate population, and
the detector decides which of those warrant a dispatch based on
whether the claimant left durable lessons behind.

The original blocked-by FEAT-059 (retro generator) is satisfied —
FEAT-059 shipped 2026-04-28; the retro is what surfaced the
release-without-done signal that drove this reshape.

## Resolution
(Filled in when status → done.)
