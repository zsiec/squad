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
priority: %s
area: %s
status: open
estimate: %s
risk: %s
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

// Options carries the optional knobs `squad new` exposes via flags or config.
// Empty fields fall through to the built-in defaults (P2 / 1h / low / <fill-in>).
type Options struct {
	Priority string
	Estimate string
	Risk     string
	Area     string
}

func New(squadDir, prefix, title string) (string, error) {
	return NewWithOptions(squadDir, prefix, title, Options{})
}

func NewWithOptions(squadDir, prefix, title string, opts Options) (string, error) {
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
	priority := nonEmpty(opts.Priority, "P2")
	estimate := nonEmpty(opts.Estimate, "1h")
	risk := nonEmpty(opts.Risk, "low")
	area := nonEmpty(opts.Area, "<fill-in>")
	now := time.Now().UTC().Format("2006-01-02")
	body := fmt.Sprintf(stubTemplate, id, title, t, priority, area, estimate, risk, now, now)
	path := filepath.Join(squadDir, "items", id+"-"+kebab(title)+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func nonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

var kebabRe = regexp.MustCompile(`[^a-z0-9]+`)

const maxKebabLen = 60

func kebab(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = kebabRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	if len(s) > maxKebabLen {
		s = strings.TrimRight(s[:maxKebabLen], "-")
		if s == "" {
			s = "untitled"
		}
	}
	return s
}
