---
id: BUG-033
title: DoR vague-acceptance-bullet verb allow-list misses common English verbs causing false positives
type: bug
priority: P2
area: internal/items
status: done
estimate: 30m
risk: low
evidence_required: [test]
created: 2026-04-27
updated: "2026-04-27"
captured_by: agent-bbf6
captured_at: 1777323490
accepted_by: agent-bbf6
accepted_at: 1777323585
references: []
relates-to: [FEAT-036]
blocked-by: []
---

## Problem

The `vague-acceptance-bullet` DoR rule in `internal/items/dor.go` (added
by FEAT-036) rejects testable AC bullets that use common English verbs
the allow-list happens not to enumerate. Today, the bullet `the
reproducer test goes from FAIL to PASS without weakening any existing
assertion` fails DoR because `goes` is not in
`vagueACBulletAllowedVerbs`, even though the proposition is concrete and
refutable. This blocks `squad accept` on items that are otherwise
well-specified, producing exactly the false-positive failure mode
FEAT-036's Notes section warned against.

## Context

`internal/items/dor.go:90` defines `vagueACBulletAllowedVerbs` — a closed
list of ~110 verbs. The comment above it (lines 86-89) explicitly invites
expansion when a real AC trips: *"Growing it further is fine when a real
AC trips on a missing verb."* This bug captures one such trip.

The missing-verb cluster is motion / state-transition vocabulary. `moves`
is present; `goes` / `go` / `went` are not. `transitions` is present;
`becomes` / `change` / `changes` are not. The corpus tuning covered "what
code does" verbs (returns, emits, validates) but missed verbs describing
observable state changes — which are exactly the verbs test-asserting
bullets reach for.

Real trip captured during BUG-032 review: a bullet "the reproducer test
goes from FAIL to PASS without weakening any existing assertion" was
filed, and `squad accept BUG-032` produced:

```
[vague-acceptance-bullet] bullet "The reproducer test (see Notes) goes
from FAIL to PASS without weakening any existing assertion." contains no
recognized verb; rephrase as a proposition (e.g., "the X rejects/returns/
validates Y")
```

The author can rephrase using `passes` or `succeeds` to clear the
check — but that is the noise-verb adaptation the FEAT-036 Notes warned
against ("agents will start adding noise verbs to clear the check"). The
right fix is to grow the legitimate-verb list, not to push authors toward
verb-laundering.

## Acceptance criteria

- [ ] `vagueACBulletAllowedVerbs` in `internal/items/dor.go` gains the missing motion / state-transition / regression cluster: `goes`, `go`, `went`, `going`, `becomes`, `become`, `change`, `changes`, `changed`, `move`, `regresses`, `regress`, `breaks`, `holds`, plus any other neutral verbs an honest scan of the corpus surfaces.
- [ ] New test case in `internal/items/dor_test.go` asserts a feature/bug bullet `the reproducer test goes from FAIL to PASS` clears the rule.
- [ ] Every previously-passing assertion in `internal/items/dor_test.go` still passes after the verb additions land.
- [ ] BUG-032 (or any item with the bullet phrasing above) accepts cleanly after the fix.

## Notes

Resist the urge to ALSO loosen the rule to "no allow-list, accept any
word ending in -s." The closed-list design is correct; the issue is only
under-coverage of one verb cluster.

Sanity check after the fix: run `squad accept --dry-run`-equivalent
against every item currently in `.squad/items/` and `.squad/done/`. The
FEAT-036 author tuned against this corpus once; re-running confirms the
verb additions did not flip any item from rejected to accepted (which
would mean we relaxed the rule too far) or vice versa.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
