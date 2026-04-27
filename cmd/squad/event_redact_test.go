package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/zsiec/squad/internal/config"
)

func TestRedact_PassthroughWhenSafeAndShort(t *testing.T) {
	cfg := redactConfig{MaxLen: 200}
	out, redacted := redact("plain target string", cfg)
	if out != "plain target string" {
		t.Errorf("out = %q; want passthrough", out)
	}
	if redacted {
		t.Errorf("redacted = true; want false")
	}
}

func TestRedact_TruncatesWhenOverMaxLen(t *testing.T) {
	cfg := redactConfig{MaxLen: 10}
	out, redacted := redact("0123456789ABCDEF", cfg)
	if len(out) != 10 {
		t.Errorf("len(out) = %d; want 10", len(out))
	}
	if out != "0123456789" {
		t.Errorf("out = %q; want first-10 prefix", out)
	}
	if redacted {
		t.Errorf("redacted = true on truncate-only path; want false (only pattern match flips the bool)")
	}
}

func TestRedact_ZeroMaxLenIsUnlimited(t *testing.T) {
	cfg := redactConfig{MaxLen: 0}
	long := strings.Repeat("a", 5000)
	out, redacted := redact(long, cfg)
	if len(out) != 5000 {
		t.Errorf("len(out) = %d; want 5000 (MaxLen=0 means unlimited)", len(out))
	}
	if redacted {
		t.Errorf("redacted = true; want false")
	}
}

func TestRedact_PatternMatchReturnsPlaceholder(t *testing.T) {
	pat := regexp.MustCompile(`(?i)password|token|secret`)
	cfg := redactConfig{Pattern: pat, MaxLen: 200}
	out, redacted := redact("auth Bearer token=abc123", cfg)
	if out != "<redacted>" {
		t.Errorf("out = %q; want <redacted>", out)
	}
	if !redacted {
		t.Errorf("redacted = false; want true on pattern match")
	}
}

func TestRedact_PatternNoMatchPreservesInput(t *testing.T) {
	pat := regexp.MustCompile(`password`)
	cfg := redactConfig{Pattern: pat, MaxLen: 200}
	out, redacted := redact("benign value", cfg)
	if out != "benign value" {
		t.Errorf("out = %q; want passthrough", out)
	}
	if redacted {
		t.Errorf("redacted = true; want false on no-match")
	}
}

func TestRedact_PatternBeatsTruncation(t *testing.T) {
	pat := regexp.MustCompile(`secret`)
	cfg := redactConfig{Pattern: pat, MaxLen: 5}
	long := strings.Repeat("a", 100) + "secret" + strings.Repeat("b", 100)
	out, redacted := redact(long, cfg)
	if out != "<redacted>" {
		t.Errorf("out = %q; want <redacted> (pattern fires before truncation)", out)
	}
	if !redacted {
		t.Errorf("redacted = false; want true")
	}
}

func TestRedact_NilPatternIsNoOp(t *testing.T) {
	cfg := redactConfig{Pattern: nil, MaxLen: 200}
	out, redacted := redact("anything goes", cfg)
	if out != "anything goes" {
		t.Errorf("out = %q; want passthrough", out)
	}
	if redacted {
		t.Errorf("redacted = true; want false")
	}
}

func TestResolveRedactConfig_EnvBeatsYAML(t *testing.T) {
	t.Setenv("SQUAD_REDACT_REGEX", "from_env")
	cfg := resolveRedactConfig(config.EventsConfig{RedactRegex: "from_yaml", MaxTargetLen: 50})
	if cfg.Pattern == nil || !cfg.Pattern.MatchString("xxx from_env xxx") {
		t.Errorf("env regex did not win; cfg.Pattern=%v", cfg.Pattern)
	}
	if cfg.MaxLen != 50 {
		t.Errorf("MaxLen = %d; want 50 (yaml MaxTargetLen still wins for the cap)", cfg.MaxLen)
	}
}

func TestResolveRedactConfig_YAMLWhenNoEnv(t *testing.T) {
	t.Setenv("SQUAD_REDACT_REGEX", "")
	cfg := resolveRedactConfig(config.EventsConfig{RedactRegex: "from_yaml", MaxTargetLen: 0})
	if cfg.Pattern == nil || !cfg.Pattern.MatchString("hit from_yaml here") {
		t.Errorf("yaml regex not applied; cfg.Pattern=%v", cfg.Pattern)
	}
	if cfg.MaxLen != defaultMaxTargetLen {
		t.Errorf("MaxLen = %d; want default %d when yaml MaxTargetLen=0", cfg.MaxLen, defaultMaxTargetLen)
	}
}

func TestResolveRedactConfig_DefaultsWhenAllEmpty(t *testing.T) {
	t.Setenv("SQUAD_REDACT_REGEX", "")
	cfg := resolveRedactConfig(config.EventsConfig{})
	if cfg.Pattern != nil {
		t.Errorf("Pattern = %v; want nil when no env and no yaml", cfg.Pattern)
	}
	if cfg.MaxLen != defaultMaxTargetLen {
		t.Errorf("MaxLen = %d; want default %d", cfg.MaxLen, defaultMaxTargetLen)
	}
}

func TestResolveRedactConfig_BadRegexFallsBackSilently(t *testing.T) {
	t.Setenv("SQUAD_REDACT_REGEX", "[unterminated")
	cfg := resolveRedactConfig(config.EventsConfig{RedactRegex: "valid_yaml"})
	// Bad env regex must not crash; fall through to yaml.
	if cfg.Pattern == nil {
		t.Fatal("Pattern is nil; expected fallback to yaml regex")
	}
	if !cfg.Pattern.MatchString("hit valid_yaml here") {
		t.Errorf("expected fallback to yaml regex; cfg.Pattern=%v", cfg.Pattern)
	}
}

func TestResolveRedactConfig_BothBadIsNoPattern(t *testing.T) {
	t.Setenv("SQUAD_REDACT_REGEX", "[bad")
	cfg := resolveRedactConfig(config.EventsConfig{RedactRegex: "(also[bad"})
	if cfg.Pattern != nil {
		t.Errorf("Pattern = %v; want nil when both env and yaml are invalid", cfg.Pattern)
	}
}
