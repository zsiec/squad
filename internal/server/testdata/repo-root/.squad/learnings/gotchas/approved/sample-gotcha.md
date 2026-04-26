---
id: GOTCHA-001
kind: gotcha
slug: sample-gotcha
title: Sample gotcha for server tests
area: server
paths:
  - internal/server/**
created: 2026-04-26
created_by: agent-test
session: test-session
state: approved
evidence:
  - tests pass
related_items:
  - BUG-100
---

# Sample gotcha

## Looks like

Server tests fail to find the learning fixture and return an empty list.

## Is

The fixture lives under a synthetic repo-root because internal/learning.Walk
joins repoRoot with .squad/learnings before walking. Subsequent server-handler
tests pass Config{LearningsRoot: "testdata/repo-root"}.
