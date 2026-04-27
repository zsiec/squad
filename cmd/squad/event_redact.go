package main

import (
	"os"
	"regexp"

	"github.com/zsiec/squad/internal/config"
)

// defaultMaxTargetLen caps an event row's `target` column when no operator
// override applies. 200 is generous for paths and short tool args, narrow
// enough that an accidental log of a long secret still ends up truncated.
const defaultMaxTargetLen = 200

// redactConfig governs the per-call behaviour of redact. A nil Pattern means
// "no regex screen, only truncate." MaxLen=0 means "no truncation."
type redactConfig struct {
	Pattern *regexp.Regexp
	MaxLen  int
}

// redact applies the operator's privacy controls to a target string before it
// is written to the agent_events ledger. Pattern is the privacy gate — when
// it matches, the input is replaced wholesale with the literal placeholder
// "<redacted>" and the redacted return flag is set. Otherwise the string is
// truncated to MaxLen bytes (zero-or-negative = unlimited) and returned
// as-is. Byte-level truncation can split a multi-byte UTF-8 codepoint;
// targets are paths and short tool args in practice and the worst case is a
// single mojibake character at the cap.
//
// Pattern wins over truncation: a value that triggers the regex never leaks
// even a prefix into the ledger.
func redact(s string, cfg redactConfig) (string, bool) {
	if cfg.Pattern != nil && cfg.Pattern.MatchString(s) {
		return "<redacted>", true
	}
	if cfg.MaxLen > 0 && len(s) > cfg.MaxLen {
		return s[:cfg.MaxLen], false
	}
	return s, false
}

// resolveRedactConfig produces a redactConfig from the layered sources, in
// priority order: SQUAD_REDACT_REGEX (env, runtime override), the per-repo
// `events.redact_regex` field, then no pattern. A regex that fails to compile
// at any tier is silently ignored — this is a fail-open recorder, not a
// gate, and shouting from inside a hook would block the agent.
func resolveRedactConfig(cfg config.EventsConfig) redactConfig {
	out := redactConfig{MaxLen: cfg.MaxTargetLen}
	if out.MaxLen <= 0 {
		out.MaxLen = defaultMaxTargetLen
	}
	if env := os.Getenv("SQUAD_REDACT_REGEX"); env != "" {
		if pat, err := regexp.Compile(env); err == nil {
			out.Pattern = pat
			return out
		}
	}
	if cfg.RedactRegex != "" {
		if pat, err := regexp.Compile(cfg.RedactRegex); err == nil {
			out.Pattern = pat
		}
	}
	return out
}
