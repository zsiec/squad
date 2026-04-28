---
title: Polish and prune from usage data
spec: agent-team-management-surface
status: open
parallelism: |
  Filled in by 'squad analyze polish-and-prune-from-usage-data' — do not hand-edit.
---

## Goal

Use the SQLite event ledger as honest evidence to (a) remove
features the data shows are unused, (b) restructure features whose
real usage doesn't match their designed shape, and (c) sharpen the
features that *are* doing real work. The ledger is the substrate;
this epic uses it for what it's actually best at — telling us what
to stop carrying.

## Observation

A 14-day audit of the global ledger surfaced a clean signal split:

**Working hard, keep investing:**

- Claim/release/done: 870 events in 7d, 95% attestation pass rate.
- Per-claim worktrees: every recent claim. Auto-fold-on-done
  silent and reliable.
- `superpowers:code-reviewer` subagent: 239 dispatches in 7d. By
  10x the most-used Agent type. Caught real bugs.
- Pre-edit touch hook: 79 entries / 64 distinct paths in 7d,
  growing daily.
- Specs/epics hierarchy: 7 specs, 16 epics, 74 items linked.
- Standup digest at `squad go` (just shipped): chat ratio
  improved from 5.5:1 auto-broadcasts vs human verbs to 3.7:1
  on the day it landed.

**Effectively dead:**

- `knock` chat verb: 1 use in 14 days.
- `answer` chat verb: 1 use in 14 days.
- `blocked` status path: **0** events in 7d, 0 in 14d. Operators
  call `release` with prose like "still blocked on FEAT-014
  schema" instead. The whole `squad blocked <ID>` state-machine
  branch fires zero times.
- `squad refine` peer-queue: 0 refinement-history blocks across
  178 done items. Comment-driven auto-refine (FEAT-062/064)
  covers every case.
- Free-form `outcome` field on `release`: massive prose sprawl
  (e.g. "natural pause point: FEAT-006 just shipped after heavy
  parallel coordination..."). Should be one of ~4 enum values
  with a separate `--note` for prose.

**Subtle gaps:**

- `learnings/` directory has only `gotchas/`; no proposed/
  approved entries exist. The propose verb is too heavy for
  what agents actually want to file.
- 42 of 231 commits have item-IDs in subjects, but ~90% are
  squad's own `chore(squad):` bookkeeping commits. The
  PM-trace convention is mostly holding; one one-line
  pre-commit guard would close the gap.

## Items in this epic

Sequenced roughly by leverage. Most are independent; a few share
state and are flagged.

### High-leverage (P2)

- **FEAT — replace blocked status with `release --reason <enum>`.**
  Drop the `blocked` state branch entirely. Add a structured
  reason enum (`released | superseded | blocked | abandoned`)
  to `squad release`, with optional `--note` for prose. Migrate
  existing free-form outcome strings via a one-time data
  migration that classifies on text patterns. Auto-postmortem
  (FEAT-061) gets a simpler trigger. Biggest cleanup payoff.

- **FEAT — delete the peer-queue refine path.** Revisits
  FEAT-065's keep-document decision in light of zero usage in
  14 days. Remove the `/api/items/:id/refine` endpoint, the
  SPA composer (was already replaced by FEAT-064's range
  composer), and the `needs-refinement` status path. Keep
  `squad refine <ID>` CLI command if it has independent users
  (verify before delete).

### Mid-leverage (P3)

- **CHORE — remove unused chat verbs (`knock`, `answer`).**
  Both have 1 use in 14 days. Update CLI, MCP, skills,
  CLAUDE.md, AGENTS.md, and the dashboard SPA's chat
  rendering. Hard-remove; the dogfood project is the only
  user.

- **FEAT — pre-commit hook flags PM-traces in commit subjects.**
  Allow `chore(squad):` prefixes through (these are
  squad-itself bookkeeping commits). Reject other subjects
  matching `BUG-NNN | FEAT-NNN | CHORE-NNN | TASK-NNN`. One
  shell hook in the scaffold.

- **FEAT — `squad doctor` reports unused features.**
  Inverts the doctor relationship: instead of agents
  reporting drift to doctor, doctor surfaces feature-usage
  gaps to operators. "3 chat verbs with 0 use in 30d —
  consider removing." Reads its own ledger. Ships the policy
  alongside the data.

- **FEAT — schema-doctor: audit unused tables/columns.**
  Several DB tables have schema bit-rot (e.g. `learnings`
  table missing despite filesystem usage; `reads` table has
  unexpected columns). Walk the schema, cross-check against
  code references, flag tables/columns with no inserts in 30d
  or no production reads.

- **FEAT — code-reviewer subagent polish.** Two concrete moves:
  (1) auto-detect leftover scratch files / unrestored patches
  from the reviewer's empirical-verify pass and fail the
  review if the working tree is dirty after; (2) add an
  explicit `blocking: true|false` field per finding so
  `squad done` can auto-detect "review has open blockers"
  without parsing prose.

### Low-leverage but cheap (P3)

- **CHORE — mention-prioritized standup digest.** When the
  mailbox has unread `@<me>` mentions, the `peers:` block at
  `squad go` should surface those above the last-touched-time
  ordering. Today the digest shows last-touch-per-peer; the
  agent has to scroll the mailbox separately to learn who's
  asking what.

- **CHORE — alignment audit of skills + CLAUDE.md after
  removals.** Once knock/answer/blocked are gone, the
  cadence/handoff/loop skills and CLAUDE.md "Chat cadence"
  table need a sweep. Generate from the live verb registry
  rather than maintaining by hand.

- **CHORE — stats panel tile cleanup after feature removals.**
  The dashboard's stats grid has tiles for blocked items and
  refinement counts that will become 0-forever after the
  removals above. Replace with the new release-reason
  breakdown and a "verbs in use" snapshot.

### Observational (no item until evidence accumulates)

- **Watch the `learnings/` pipeline.** FEAT-061's auto-postmortem
  is the one mechanism that mechanically produces learnings.
  If it produces real artifacts over the next month, the
  propose→approve→auto-load pipeline is right and `propose`
  was just under-used as a manual UX. If `learnings/` stays
  empty, the whole pipeline needs rework or removal. File as
  an item only after the data tells us which way.

## Sequencing

The two P2 items are independent and can ship in parallel. The
chat-verb removal is a prerequisite for the alignment-audit and
stats-panel-cleanup items (both depend on the verb registry being
final). Pre-commit hook, doctor-usage-gap, schema-doctor, and
code-reviewer polish are all standalone — pick by interest or by
parallelism.

## Non-goals

- New features. The instruction explicitly framed this as polish
  and pruning. Anything that adds surface area belongs in a
  different epic.
- Provider-neutral abstractions. The Claude-Code-shaped runtime
  (hooks, skills, MCP) is the moat; widening this epic to
  "support other agents" would be the wrong sweep.
- Hosted dashboard / cloud surface. Local-first is the moat;
  any item that touches deployment shape is out of scope here.

## Anti-items (recorded so they don't get filed later)

- "Migrate `~/.squad/global.db` to Postgres for multi-machine
  collaboration." Eliminates the local-first claim. Different
  product.
- "Add a chat verb for X." The data says we have too many
  verbs, not too few. The polish direction is fewer-and-louder,
  not more-and-thinner.
- "Build an agent marketplace for skills." Curation problem
  with no moat connection. If skill auto-loading proves
  insufficient the next move is smarter loading rules, not a
  registry.
