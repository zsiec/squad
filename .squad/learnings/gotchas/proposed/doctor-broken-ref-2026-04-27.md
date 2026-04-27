---
id: gotcha-2026-04-27-doctor-broken-ref-2026-04-27
kind: gotcha
slug: doctor-broken-ref-2026-04-27
title: doctor finding broken_ref
area: hygiene
paths:
  - internal/hygiene/**
created: 2026-04-27
created_by: agent-401f
session: 
state: proposed
evidence: []
related_items: []
tags:
  - doctor-finding
  - doctor-code-broken-ref
---

## Looks like

`squad doctor` found 8 broken_ref finding(s):

- item TASK-001 references missing file internal/items/parse.go
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-001-body-rewrite-parser-writefeedback-movefeedbacktohistory.md — fix or remove the reference
- item TASK-002 references missing file internal/store/schema.sql
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-002-allow-needs-refinement-in-items-status-db-schema-check-migra.md — fix or remove the reference
- item TASK-021 references missing file internal/store/schema.sql
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-021-agent-events-table-migration.md — fix or remove the reference
- item TASK-022 references missing file cmd/squad/event.go (new)
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-022-squad-event-record-internal-cli-verb.md — fix or remove the reference
- item TASK-022 references missing file cmd/squad/main.go (cobra root for command registration)
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-022-squad-event-record-internal-cli-verb.md — fix or remove the reference
- item TASK-026 references missing file internal/server/web/agents.js
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-026-spa-agent-detail-drawer-scaffolding-click-handler-in-agents.md — fix or remove the reference
- item TASK-027 references missing file internal/server/web/agents.js
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-027-spa-timeline-view-rendering-with-kind-filters-localstorage-p.md — fix or remove the reference
- item TASK-028 references missing file internal/server/web/agents.js
  fix: edit /Users/zsiec/dev/squad/.squad/done/TASK-028-sse-live-update-channel-for-agent-timeline-drawer.md — fix or remove the reference


## Is

_What it actually is, with the evidence that proves it._

## So

_The corrective action future agents should take._
