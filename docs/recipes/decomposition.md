# Recipe: Decomposing a spec into items

## Who this is for

You have a chunk of work that's bigger than one item. A spec — a paragraph or two of motivation, an acceptance list, the non-goals — captures the whole thing, but it's too large for any single claim. You want to draft a handful of captured items that, taken together, deliver the spec, then triage them down to what actually ships.

## The flow at a glance

```
squad spec-new my-spec ──► (edit) ──► squad decompose my-spec ──► squad inbox --parent-spec=my-spec
                                                                          │
                                                                          ├─► accept the keepers
                                                                          └─► reject the noise
```

Decomposition produces drafts in the `captured` state. Nothing lands as `open` automatically; you triage each one through the same DoR check as any other captured item.

## Walkthrough

### 1. Author a spec

```bash
squad spec-new auth-rework "rebuild authentication around OIDC"
```

This writes `.squad/specs/auth-rework.md` with `title`, `motivation`, `acceptance`, `non_goals`, and `integration` frontmatter. Open it and fill in the prose. Specs are read by the decomposer, by reviewers, and by future you trying to remember what the goal was — write them for that audience.

The minimum useful spec has:

- **motivation** — why this work exists; what changes for users when it's done.
- **acceptance** — the spec-level done criteria. Each item should map to at least one acceptance line.
- **non_goals** — explicit out-of-scope items. The decomposer sees these and won't produce drafts for them.

### 2. Decompose

```bash
squad decompose auth-rework
```

The command reads the spec, asks the model to suggest a parallel decomposition, and writes one captured item per suggested chunk under `.squad/items/`. Each draft has:

- `kind` (usually `feat` or `task`, sometimes `chore` or `infra`)
- a generated title
- `parent_spec: auth-rework` in frontmatter
- a stub `## Problem` section pulled from the spec context
- a stub `## Acceptance criteria` section, often empty or sparse

Add `--print-prompt` to see the prompt the decomposer is sending, useful for debugging when the output isn't what you expected.

From a Claude Code session, the same operation runs through `/squad:squad-decompose auth-rework`.

### 3. Triage the resulting drafts

```bash
squad inbox --parent-spec=auth-rework
```

This filters the inbox to the items you just created. Walk down the list. For each one:

- **Keep it?** Edit the file to flesh out the AC, set `area:`, and run `squad accept <id>`.
- **Drop it?** `squad reject <id> --reason "..."` — usually because the decomposer suggested a chunk that's actually two specs, or because it overlaps with existing work.
- **Defer it?** Leave it in the inbox; it stays captured until you come back. The doctor will flag it after a week.

### 4. Accept the keepers

```bash
squad accept FEAT-014
squad accept FEAT-015
squad accept TASK-022
```

Each acceptance runs the Definition of Ready check. Drafts from the decomposer rarely pass on the first try — they tend to need an `area:` set and the AC sharpened. That's the point of the captured state: the decomposer's first pass is a starting point, not a finished item.

After acceptance, the items show up in `squad next` and any agent can claim them.

### 5. Reject the noise

```bash
squad reject FEAT-016 --reason "merged into FEAT-014; same surface area"
squad reject TASK-023 --reason "non-goal per spec; out of scope"
```

The reasons land in `.squad/inbox/rejected.log` so the audit trail survives. Reading the log later — `squad inbox --rejected` — sometimes catches a pattern: if every decomposition keeps suggesting the same chunk that you keep rejecting, the spec itself probably needs editing to make the non-goal explicit.

## Running this from Claude Code

> *"Decompose the auth-rework spec into items."*

Claude calls `squad_decompose auth-rework` and reports back the list of drafts with their ids.

> *"Show me the inbox for that spec."*

Claude calls `squad_inbox` filtered by parent spec. You can ask Claude to draft AC for each one inline before accepting.

## A note on iteration

Don't expect a single decomposition to land the perfect set of items. Treat the first pass as a brainstorm: keep the obvious wins, reject the obvious noise, edit the spec to clarify what was ambiguous, and re-run `squad decompose` if the gaps are material. The cost of a re-run is one model call and another triage pass; the cost of accepting a bad item is a wasted claim.

## See also

- [concepts/intake.md](../concepts/intake.md) — the captured/open model.
- [recipes/triage.md](triage.md) — the triage loop in detail.
- [reference/commands.md](../reference/commands.md) — `squad spec-new`, `squad decompose`, and the inbox verbs.
