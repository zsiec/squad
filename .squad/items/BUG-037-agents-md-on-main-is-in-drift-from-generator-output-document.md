---
id: BUG-037
title: AGENTS.md on main is in drift from generator output; documentation-contract epic landed without regenerating
type: bug
priority: P2
area: docs
status: open
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777323804
accepted_by: web
accepted_at: 1777325381
references: []
relates-to: []
blocked-by: []
---

## Problem

The documentation-contract epic (FEAT-049 / FEAT-050 / CHORE-015) closed
without ever regenerating `AGENTS.md` from the new generator. The file
on main is the old hand-edited prose (~287 lines, last touched in
`c76e6e5 dogfood: commit squad backlog` on 2026-04-26). Running
`squad scaffold agents-md --check` against current main exits 2 with
`AGENTS.md drift: file does not match \`squad scaffold agents-md\`
output`. The pre-commit hook from CHORE-015 will reject any future
commit that stages a change to `AGENTS.md` until somebody regenerates.

Two consequences:

1. The contract the epic stated — "AGENTS.md becomes a generated
   artifact, computed from ledger state at scaffold time" — is
   not actually in effect on main; the file is still hand-edited content.
2. Anyone currently editing `AGENTS.md` for any reason hits the hook
   without an obvious cause; the ledger does not record this trap.

## Context

Reproduce:

```
$ go run ./cmd/squad scaffold agents-md --check
AGENTS.md drift: file does not match `squad scaffold agents-md` output. Regenerate before commit
exit status 2
```

Cross-check the closing commits:

```
$ git log --pretty=format:'%h %s' -- AGENTS.md
c76e6e5 dogfood: commit squad backlog       # 2026-04-26
```

The CHORE-015 closing commit (`2bc53a1`) on 2026-04-27 does not touch
`AGENTS.md`; the FEAT-049 closing commit (`6edc950`) does not touch
`AGENTS.md`. Net effect: the epic shipped with the artifact in
permanent drift relative to the contract it imposes.

## Acceptance criteria

- [ ] Decide and document on the epic itself: does the operating
      manual content currently in the hand-edited `AGENTS.md` (§0–§12,
      ~250 lines) move into `CLAUDE.md` (the only hand-edited file
      per the epic), or is it discarded? The current generator output
      does not include it.
- [ ] After that decision lands, run `squad scaffold agents-md` and
      commit the regenerated file so `squad scaffold agents-md
      --check` exits 0 on a fresh checkout of main.
- [ ] Add a CI check (or extend the existing pre-commit hook test)
      that the committed `AGENTS.md` passes `--check` against
      generator output — so this regression cannot recur silently.

## Notes

The hand-edited operating manual sections in current `AGENTS.md` are
load-bearing for new contributors — bullets like "post often, post
small, post honestly" and the §1–§12 cadence guide are the practical
how-to. Discarding them when AGENTS.md becomes generator-only is a
real loss; preserving them in `CLAUDE.md` is probably the right call.

Related: BUG-035 and BUG-036 describe quality issues with the
generator output itself; they should land before the regeneration so
the committed `AGENTS.md` is actually useful.
