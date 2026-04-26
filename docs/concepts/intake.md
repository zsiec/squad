# Intake

## Why a two-state model

The original loop assumed every filed item was immediately ready: someone with full context typed the AC, picked an area, and the next session could claim it. That works when filing and shaping are the same act. It does not work when filing happens fast — *"this caching layer scares me"*, *"export is broken on Safari"*, *"split the auth spec into items"* — and shaping is something you sit down to do later.

The cost of conflating the two is real. Half-baked items pile into `squad next`. An agent claims one, discovers the AC is "make it work right," and has to drop the claim or guess. Every guess is wasted work; every drop is friction.

Squad now models capture and acceptance as two states:

- **`captured`** — filed but not yet ready to claim. Has at minimum a title and a kind. May be missing area, AC, or a clear problem statement. Lives in `.squad/items/` like any other item; excluded from `squad next` and from the priority stack.
- **`open`** — passed the Definition of Ready. Eligible to claim. Behaves identically to pre-intake `status: open` items — nothing about the working loop changes.

Capture-fast, refine-later mirrors the GTD inbox pattern: get the thought out of your head into a trusted system, decide whether it deserves attention later. Items that never make it to `open` are not failures; they are signal that a thought wasn't worth the work it would have taken to file before.

## Definition of Ready

Promotion from `captured` to `open` runs through a hardcoded check (see `internal/items/dor.go`). Three rules, no configuration knob:

1. **`area-set`** — `area:` frontmatter is set to a real value, not empty and not `<fill-in>`.
2. **`acceptance-criterion`** — the body has at least one `- [ ]` checkbox under a `## Acceptance criteria` header.
3. **`title-or-problem`** — either the title is more than five words long *or* the `## Problem` section has a non-empty body. A one-word title with no problem statement is rejected; either you describe the symptom in the title or you describe it in the body.

These are intentionally minimal. They catch the common failure modes — empty AC, missing area, "fix the thing" titles — without forcing the author to fill out a template. Anything stricter belongs in your team's `AGENTS.md`, not in the binary.

`squad ready --check FEAT-001` runs the rules without promoting; useful for "does this pass yet?" loops while you edit. `--strict` makes the command exit non-zero on any violation, suitable for CI.

## The four surfaces

The intake model is exposed through four parallel surfaces. Pick whichever matches how you're working:

- **CLI verbs** — `squad new`, `squad inbox`, `squad accept`, `squad reject`, `squad ready --check`, `squad decompose`. The canonical surface; everything else is built on top.
- **MCP tools** — `squad_new`, `squad_inbox`, `squad_accept`, `squad_reject`, `squad_decompose`. What Claude Code calls when you describe the action in natural language.
- **TUI inbox view** — the 12th view in `squad tui`. Lists captured items, supports accept and reject inline, surfaces DoR violations as you arrow through.
- **Slash commands** — `/squad-capture` files a captured item from anywhere in a Claude Code session; `/squad-decompose <spec>` drafts items from a spec.

All four converge on the same store. Filing through the slash command and accepting from the TUI is fine; filing through `squad new` and accepting through MCP is fine.

## Provenance fields

Every captured item records who filed it and when. Acceptance and rejection log the same. The fields live in frontmatter:

- **`captured_by`** / **`captured_at`** — agent id and timestamp at file time. Set by `squad new` automatically; never edit by hand.
- **`accepted_by`** / **`accepted_at`** — set when the item passes DoR and moves to `open`.
- **`parent_spec`** — slug of the spec this item was decomposed from, if any. `squad decompose auth-rework` sets `parent_spec: auth-rework` on every draft it produces, so you can later filter `squad inbox --parent-spec=auth-rework` for triage.

Provenance is not a feature for end users to read. It exists so the audit trail survives — *"who captured this and when?"*, *"which spec did this come from?"* — without anyone having to remember.

## Rejection: log-then-delete

Reject is permanent on the file but durable in the log. `squad reject FEAT-007 --reason "duplicate of FEAT-003"` deletes the item file and appends a row to `.squad/inbox/rejected.log` with the id, title, reason, agent, and timestamp. The reason is required — there is no way to reject anonymously.

Rejected items don't clutter `squad inbox` by default. `squad inbox --rejected` reads the log when you need to review what got dropped (most useful for "did I really mean to reject this?" second-guessing or for spotting reject-loops where the same idea keeps getting filed and dropped).

There is no un-reject. If you reject by mistake, file a fresh `squad new` with the same content; the captured/accepted timestamps reset.

## Backward compatibility

If your repo already has items with `status: open` and no `captured_by` field, nothing changes for them. The DoR check runs only on the `captured → open` transition; an item that was already `open` before the intake model shipped stays `open` and stays claimable. No migration is needed; no items are silently re-categorized.

The `--ready` flag on `squad new` (which skips the captured state and goes straight to `open` if the body passes DoR) preserves the pre-intake "file ready, claim immediately" workflow for users who don't want the inbox layer. The intake model is opt-in by default of behavior, not opt-in by configuration.

## See also

- [recipes/triage.md](../recipes/triage.md) — walkthrough of capture, accept, reject.
- [recipes/decomposition.md](../recipes/decomposition.md) — spec-to-items workflow.
- [reference/commands.md](../reference/commands.md) — full command reference for the new verbs.
