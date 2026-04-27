---
spec: auto-refine-inbox
status: open
parallelism: |
  Foundation (TASK: items.AutoRefineApply primitive + Item frontmatter fields)
  blocks every other item — the MCP tool, the server endpoint, the SPA, and
  the inbox JSON propagation all need the primitive and the parser changes
  before they can land. After the foundation, the four mid-tier items can
  land in any order: (a) MCP tool wires the primitive, (b) inbox JSON
  surfaces the new fields, (c) SPA button + spinner + badge, (d) server
  endpoint with subprocess + narrow MCP config. The integration test
  blocks-by everything visible (foundation + MCP tool + server endpoint).
  Docs land last.
---

## Goal

Implement the auto-refine-inbox spec end-to-end — items primitive,
frontmatter fields, MCP tool, server endpoint with subprocess, narrow MCP
config, SPA button + badge, inbox JSON propagation, and an integration test
— so a reviewer can click Auto-refine on a captured inbox row and watch the
body get rewritten by the local claude CLI into a real Problem / Context /
AC body that satisfies DoR, ready for the human Accept click.

## Child items

The implementation tasks are filed as child items, each with `epic:
auto-refine-inbox` in their frontmatter. They are listed and tracked via
`squad next` once accepted.
