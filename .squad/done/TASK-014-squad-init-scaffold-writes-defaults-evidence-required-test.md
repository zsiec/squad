---
id: TASK-014
title: squad init scaffold writes defaults.evidence_required:[test]
type: task
priority: P2
area: scaffold
status: done
estimate: 30m
risk: low
created: 2026-04-26
updated: "2026-04-26"
captured_by: agent-bbf6
captured_at: 1777245992
accepted_by: agent-bbf6
accepted_at: 1777245992
epic: feature-uptake-nudges
evidence_required: [test]
references:
  - internal/scaffold/templates
  - internal/scaffold/scaffold.go
relates-to: []
blocked-by:
  - TASK-012
---

## Problem

When a user runs `squad init` on a fresh repo, the generated `.squad/config.yaml` has no `defaults.evidence_required` field, so the attestation gate stays dormant until the maintainer notices and edits the config by hand. Default the field on so adopting repos get the gate from day one — they can opt out by setting it to `[]`.

## Context

Templates live under `internal/scaffold/templates/` (run `find internal/scaffold/templates -name '*.tmpl'` to confirm the config template's filename). The scaffold writer is in `internal/scaffold/scaffold.go` / `write.go`. There are existing scaffold tests (`scaffold_test.go`, `write_test.go`) that assert on generated content.

## Acceptance criteria

- [ ] The scaffolded `.squad/config.yaml` template includes a `defaults:` block with `evidence_required: [test]` (with a one-line comment explaining the field).
- [ ] If the existing template already has a `defaults:` block, this work appends the line; if not, the block is added.
- [ ] Unit test: a fresh `scaffold.Write(tmp, scaffold.Options{})` produces a config containing `evidence_required: [test]`.
- [ ] `go test ./internal/scaffold/...` passes; trailing `ok` line pasted.

## Notes

- Blocked-by TASK-012 because the field has to exist on the `Defaults` struct before scaffolding writes it — otherwise `config.Load` would treat it as an unknown field (yaml.v3 will silently ignore it, but we'd still rather land the field first to keep semantics tight).
- Operators with existing repos can copy the line into their `.squad/config.yaml` by hand; we don't auto-migrate.

## Resolution

### Fix

`internal/scaffold/templates/config.yaml.tmpl` — appended `evidence_required: [test]` to the existing `defaults:` block, with a one-line comment explaining replace-not-merge semantics inherited from TASK-012's `Done()` fallback.

### Test

`internal/scaffold/scaffold_test.go::TestConfigTemplate_RendersAllKnobs` — new assertion that the rendered template contains `evidence_required: [test]`. Verified RED before the template change, GREEN after.

> Note: AC #3 mentions `scaffold.Write(tmp, scaffold.Options{})` but no such API exists in this codebase. The actual scaffold flow goes `Templates.ReadFile` → `Render(string(raw), Data{})`. The new assertion plugs into the existing `TestConfigTemplate_RendersAllKnobs` which exercises that same path — same coverage, idiomatic location.

### Evidence

```
$ go test ./internal/scaffold/...
ok  	github.com/zsiec/squad/internal/scaffold	0.529s
```

Full `go test ./...` passes (0 FAIL lines).

### AC verification

- [x] Scaffolded `.squad/config.yaml` defaults block includes `evidence_required: [test]` with explanatory comment.
- [x] Line appended to existing block (no new block created).
- [x] Unit test asserts the rendered template contains the new line.
- [x] `go test ./internal/scaffold/...` passes.
