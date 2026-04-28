---
id: FEAT-069
title: squad doctor reports feature usage gaps from the ledger
type: feature
priority: P3
area: hygiene
status: open
estimate: 2h
risk: low
evidence_required: [test]
created: 2026-04-28
updated: "2026-04-28"
captured_by: agent-401f
captured_at: 1777351789
accepted_by: web
accepted_at: 1777352040
references: []
relates-to: []
blocked-by: []
epic: polish-and-prune-from-usage-data
---

## Problem

`squad doctor` today checks for stale claims, ghost agents, and
filesystem drift — agent-side correctness signals. It doesn't tell
the *operator* which features have gone unused. The polish-and-
prune analysis that motivated this epic took ~30 minutes of manual
SQL probing on the global ledger; that work should be one
`squad doctor --feature-usage` invocation.

## Context

Inverts the doctor relationship: today, doctor reports drift to
agents; this adds a path where doctor reports usage gaps to
operators. Reads its own ledger (no new data sources) and emits
findings against a small set of policies:

- Chat verbs with <N uses in the last 30d.
- CLI subcommands not invoked in 30d (cross-reference
  `agent_events` tool column against the cobra command list).
- MCP tools registered but not called in 30d.
- DB tables with no inserts in 30d (cross-reference with
  `schema-doctor` work in FEAT-070 — coordinate but don't
  duplicate).

## Acceptance criteria

- [ ] `squad doctor --feature-usage` runs and emits a markdown
      block with the chat verbs, CLI subcommands, MCP tools, and
      tables under their respective usage thresholds. Default
      window 30 days; `--days N` overrides.
- [ ] Output is suppressible (`--quiet` or no flag for the
      default `squad doctor` run that doesn't include this slice).
      Goal is the operator opts in, not "more noise on every
      run".
- [ ] Threshold values are config-driven
      (`hygiene.feature_usage.{verb_min,cli_min,mcp_min}`),
      defaults pin the analysis from this epic (verb_min=5/30d,
      cli_min=1/30d, mcp_min=1/30d).
- [ ] The output includes a one-line *recommendation* per finding
      — e.g. "chat verb `knock` had 1 use in 30d (threshold 5)
      → consider removing or merging into `ask`".
- [ ] A test seeds a fixture ledger with known-low-usage features
      and asserts the report names them.
- [ ] A test asserts the report does NOT name features above the
      threshold (no false positives).

## Notes

- Companion to the manual analysis that produced this epic. The
  point is to make the analysis self-service rather than a
  one-off audit.
- The recommendation strings are intentionally rule-based and
  short; this is policy, not LLM-generated.

## Resolution
(Filled in when status → done.)
