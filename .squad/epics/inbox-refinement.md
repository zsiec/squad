---
spec: inbox-refinement
status: done
parallelism: |
  Foundation (FEAT-PARSER) blocks all server + CLI work. Server endpoints
  can land in any order once the parser exists. CLI verbs can land in
  any order once the parser exists. SPA work depends on the /refine
  endpoint. Docs land last. See child items' blocked-by lists.
---

## Goal

Implement the inbox refinement loop end-to-end — parser, three HTTP endpoints,
three CLI verbs, SPA composer, integration test, and docs — so that a captured
item can be sent back for refinement, edited by a peer agent, and re-captured
to the inbox for human review.

## Child items

The 11 implementation tasks are filed as child items, each with `epic:
inbox-refinement` in their frontmatter. They are listed and tracked via
`squad next` once accepted.
