---
id: BUG-034
title: auto-refine-inbox spec YAML breaks (acceptance bullet has unquoted colon-space)
type: bug
priority: P2
area: docs
status: done
estimate: 1h
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-afcd
captured_at: 1777323710
accepted_by: web
accepted_at: 1777325376
references: []
relates-to: []
blocked-by: []
---

## Problem

`.squad/specs/auto-refine-inbox.md` fails to parse: `specs.Parse` returns
`yaml: unmarshal errors: line 15: cannot unmarshal !!map into string`.
The third bullet under `acceptance:` contains the substring
`` `auto_refined_by: claude` `` — the embedded `: ` (colon-space) makes
YAML treat the plain scalar as a map. As a side effect the spec is
silently dropped from the generated `AGENTS.md` Specs section and from
`docs/specs.md` (FEAT-049 / FEAT-050): only 6 of 7 specs render.

## Context

Reproduce:

```
go run ./cmd/squad scaffold agents-md  # then grep '\.squad/specs/' AGENTS.md → 6 entries, auto-refine-inbox missing
```

Or, in `internal/specs/`:

```go
specs.Parse(".squad/specs/auto-refine-inbox.md")
// → decode spec ...: yaml: unmarshal errors:
//      line 15: cannot unmarshal !!map into string
```

`specs.Walk` (`internal/specs/specs.go:71`) skips parse failures
silently — so the operator gets no warning beyond the missing entry
(by design: doctor is the surfacing path, but the AGENTS.md generator
does not consult doctor).

## Acceptance criteria

- [ ] `.squad/specs/auto-refine-inbox.md` parses cleanly via
      `specs.Parse` (no YAML error).
- [ ] After `squad scaffold agents-md`, the generated `AGENTS.md` lists
      all 7 specs from `.squad/specs/` including auto-refine-inbox.
- [ ] After `squad scaffold doc-index`, `docs/specs.md` lists all 7 specs.
- [ ] A regression test in `internal/specs/` (or scaffold) parses every
      `.squad/specs/*.md` and fails the suite if any spec is dropped —
      so a future YAML typo in any spec does not silently disappear from
      the generated index.

## Notes

Likely fix: quote the bullet (`"..."` or `'...'`) or replace `: ` with
`:` (no space) inside the backticks. The simplest portable fix is to
quote the offending bullet.

The same failure mode applies to any future spec bullet whose plain
scalar contains `: ` — a long-term fix is to either always-quote
bullets in the squad-new spec template or have `specs.Walk` log a
warning when it skips a malformed file.
