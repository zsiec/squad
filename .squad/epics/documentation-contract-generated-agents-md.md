---
title: Documentation contract — generated AGENTS.md
spec: agent-team-management-surface
status: open
parallelism: parallel
dependencies:
  - Refinement and contract hardening
  - Observation to knowledge pipeline
intake_session: intake-20260427-44256e4424c4
---

## Body

The repo currently has two contract files at the root: `CLAUDE.md` and
`AGENTS.md`. Both are hand-edited prose. That works for `CLAUDE.md` —
it is the durable contract: conventions, reading order, the load-bearing
section that every contributor reads. It does not work for `AGENTS.md`,
which is supposed to tell an agent what is in flight, what is ready, and
what was recently done. That information lives in the ledger, and the
moment it is duplicated into prose it starts to drift.

The principle this epic establishes:

- `CLAUDE.md` is the only hand-edited contract file. Conventions, reading
  order, bootstrap stance — all human-curated.
- `AGENTS.md` becomes a generated artifact, computed from ledger state at
  scaffold time. Top ready items, in-flight claims, recent done, active
  specs/epics index — all derived, never typed.
- A pre-commit hook enforces the split: `AGENTS.md` cannot be hand-edited;
  `CLAUDE.md` is unaffected.

Three items make this concrete:

1. **FEAT-049** — `squad scaffold agents-md` writes `AGENTS.md` from ledger
   state. The generator is the source of truth for the file's body.
2. **FEAT-050** — `squad scaffold doc-index` writes `docs/specs.md` and
   `docs/epics.md` as auto-rendered indexes. Specs and epics are markdown
   files today with no index page; this fills that gap.
3. **CHORE-015** — Adds the do-not-edit banner to the generated `AGENTS.md`
   and a pre-commit hook that fails if the file has drifted from current
   ledger state. CHORE-015 depends on FEAT-049 because the hook needs the
   generator's output to compare against.

### Why this depends on Epic B and Epic C

The generated `AGENTS.md` reads item bodies and the learnings index. If
item bodies are still stubs (the problem Epic B — Refinement and contract
hardening — fixes), the rendered output is a wall of placeholder prose
saying "What is wrong / what doesn't exist." If the learnings index is
empty or unrendered (the problem Epic C — Observation to knowledge
pipeline — fixes), the "what we know" section of `AGENTS.md` is blank.

Either gap on its own makes the generated artifact thin and the contract
weaker than the hand-edited version we are replacing. So this epic ships
after B and C — not because the code depends on them, but because the
output quality does.
