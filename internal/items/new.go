package items

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var typeByPrefix = map[string]string{
	"BUG":   "bug",
	"FEAT":  "feature",
	"TASK":  "task",
	"CHORE": "chore",
	"DEBT":  "tech-debt",
	"BET":   "bet",
}

const stubTemplate = `---
id: %s
title: %s
type: %s
priority: P2
area: <fill-in>
status: open
estimate: 1h
risk: low
created: %s
updated: %s
references: []
relates-to: []
blocked-by: []
---

## Problem
What is wrong / what doesn't exist. 1–3 sentences.

## Context
Why this matters. Where in the codebase it lives. What's been tried.

## Acceptance criteria
- [ ] Specific, testable thing 1
- [ ] Specific, testable thing 2

## Notes
Optional design notes. Trade-offs considered. Pointers to related items.

## Resolution
(Filled in when status → done.)
What changed, file references, anything a future maintainer needs to know.
`

func New(squadDir, prefix, title string) (string, error) {
	w, err := Walk(squadDir)
	if err != nil {
		return "", err
	}
	id, err := NextID(prefix, w)
	if err != nil {
		return "", err
	}
	t, ok := typeByPrefix[prefix]
	if !ok {
		t = strings.ToLower(prefix)
	}
	now := time.Now().UTC().Format("2006-01-02")
	body := fmt.Sprintf(stubTemplate, id, title, t, now, now)
	path := filepath.Join(squadDir, "items", id+"-"+kebab(title)+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

var kebabRe = regexp.MustCompile(`[^a-z0-9]+`)

func kebab(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = kebabRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	return s
}
