---
title: Auth rework
motivation: |
  Users have reported login loops on slow networks. The current redirect
  chain assumes an instant session-cookie handshake.
acceptance:
  - login redirects to /home
  - sessions expire after 30 days
non_goals:
  - 2FA
  - SSO
integration:
  - Touches internal/auth/* and the login handler.
  - Coordinates with the session middleware.
---

## Background

Freeform prose lives below the frontmatter.
