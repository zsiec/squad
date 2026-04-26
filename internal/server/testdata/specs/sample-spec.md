---
title: Sample spec for server tests
motivation: Server tests need a real spec on disk to walk
acceptance:
  - GET /api/specs returns this spec
  - GET /api/specs/sample-spec returns full body
non_goals:
  - Production scenarios
integration:
  - Used by internal/server tests only
---

# Sample spec

This is a sample spec body used by internal/server tests.
