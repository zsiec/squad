package items

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var typeByPrefix = map[string]string{
	"BUG":   "bug",
	"FEAT":  "feature",
	"TASK":  "task",
	"CHORE": "chore",
	"DEBT":  "tech-debt",
	"BET":   "bet",
}

// stubTemplate uses %s for the title placeholder; we feed yaml-quoted output
// (`"foo: bar"` etc.) so titles containing colons, newlines, leading dashes,
// or other YAML-special characters can't poison the frontmatter.
const stubTemplate = `---
id: %s
title: %s
type: %s
priority: %s
area: %s
status: %s
estimate: %s
risk: %s
evidence_required: %s
created: %s
updated: %s
captured_by: %s
captured_at: %d
accepted_by: %s
accepted_at: %d
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
	Priority   string
	Estimate   string
	Risk       string
	Area       string
	Ready      bool
	CapturedBy string
}

func New(squadDir, prefix, title string) (string, error) {
	return NewWithOptions(squadDir, prefix, title, Options{})
}

func NewWithOptions(squadDir, prefix, title string, opts Options) (string, error) {
	var path string
	err := withItemsLock(squadDir, func() error {
		w, err := Walk(squadDir)
		if err != nil {
			return err
		}
		id, err := NextID(prefix, w)
		if err != nil {
			return err
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
		nowUnix := time.Now().Unix()
		status := "captured"
		acceptedBy := ""
		acceptedAt := int64(0)
		if opts.Ready {
			status = "open"
			acceptedBy = opts.CapturedBy
			acceptedAt = nowUnix
		}
		body := fmt.Sprintf(stubTemplate,
			id, yamlInline(title), t, priority, area, status, estimate, risk,
			defaultEvidenceForType(prefix), now, now,
			yamlInline(opts.CapturedBy), nowUnix, yamlInline(acceptedBy), acceptedAt,
		)
		path = filepath.Join(squadDir, "items", id+"-"+kebab(title)+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(body), 0o644)
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

func defaultEvidenceForType(prefix string) string {
	switch prefix {
	case "BUG", "FEAT", "TASK":
		return "[test]"
	default:
		return "[]"
	}
}

func nonEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

// yamlInline returns a yaml-safe single-line representation of s (no trailing
// newline). Lets the stub template interpolate any title without risking a
// frontmatter that yaml.Unmarshal then rejects.
func yamlInline(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return strconvQuote(s)
	}
	return strings.TrimRight(string(out), "\n")
}

// strconvQuote falls back to a Go-quoted string if yaml.Marshal somehow
// errors. Not strictly YAML, but Parse will fail loudly rather than silently
// produce a half-broken file.
func strconvQuote(s string) string {
	return fmt.Sprintf("%q", s)
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
