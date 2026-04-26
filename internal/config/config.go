// Package config loads .squad/config.yaml — squad's per-repo settings for
// hygiene cadence, WIP caps, verification gates, and similar knobs.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	IDPrefixes   []string           `yaml:"id_prefixes"`
	Agent        AgentConfig        `yaml:"agent"`
	Defaults     Defaults           `yaml:"defaults"`
	Verification VerificationConfig `yaml:"verification"`
	Hygiene      HygieneConfig      `yaml:"hygiene"`
	Touch        TouchConfig        `yaml:"touch"`
}

// TouchConfig controls how the pre-edit touch-check hook reacts when an Edit
// targets a path another agent is currently touching. Default empty =
// warn-only via permissionDecision: "ask"; set Enforcement to "deny" plus a
// list of EnforcementPaths globs to block edits on red-flag files.
type TouchConfig struct {
	Enforcement      string   `yaml:"enforcement"`
	EnforcementPaths []string `yaml:"enforcement_paths"`
}

const (
	TouchEnforcementWarn = "warn"
	TouchEnforcementDeny = "deny"
)

type AgentConfig struct {
	// ClaimConcurrency caps how many items a single agent can hold open at
	// once. 0 means "use the default" (DefaultClaimConcurrency below).
	// Set to a large number (e.g. 100) to effectively disable.
	ClaimConcurrency int `yaml:"claim_concurrency"`
}

type HygieneConfig struct {
	// StaleClaimMinutes is the threshold past which a claim's last_touch is
	// considered stale and eligible for auto-reclaim. 0 = use default.
	StaleClaimMinutes int `yaml:"stale_claim_minutes"`

	// SweepOnEveryCommand toggles the post-command hygiene runner. nil
	// (omitted) defaults to true; set to false to disable without setting
	// SQUAD_NO_HYGIENE per-invocation.
	SweepOnEveryCommand *bool `yaml:"sweep_on_every_command"`
}

// DefaultClaimConcurrency is the cap applied when config doesn't override.
// Documented as 1 since Phase 6; QA round 4 surfaced that it wasn't enforced.
const DefaultClaimConcurrency = 1

// DefaultStaleClaimMinutes matches the value advertised in the scaffold and
// reference docs. QA round 5 surfaced a 30/60 mismatch between code and docs;
// resolved by adopting the documented 60-minute default.
const DefaultStaleClaimMinutes = 60

type Defaults struct {
	Priority string `yaml:"priority"`
	Estimate string `yaml:"estimate"`
	Risk     string `yaml:"risk"`
	Area     string `yaml:"area"`
}

type VerificationConfig struct {
	PreCommit []VerificationCmd `yaml:"pre_commit"`
}

type VerificationCmd struct {
	Cmd      string `yaml:"cmd"`
	Evidence string `yaml:"evidence"`
}

var defaultPrefixes = []string{"BUG", "FEAT", "TASK", "CHORE"}

// ValidateTouch returns human-readable warnings for a TouchConfig. An empty
// slice means the config is well-formed. Unknown enforcement values are
// flagged so a typo like "denied" or "warning" doesn't silently degrade to
// warn-only mode.
func ValidateTouch(cfg TouchConfig) []string {
	var warns []string
	switch cfg.Enforcement {
	case "", TouchEnforcementWarn, TouchEnforcementDeny:
	default:
		warns = append(warns, fmt.Sprintf(
			"touch.enforcement=%q is not recognized; valid values are %q|%q (defaulting to warn)",
			cfg.Enforcement, TouchEnforcementWarn, TouchEnforcementDeny,
		))
	}
	return warns
}

func Load(repoRoot string) (Config, error) {
	cfg := Config{IDPrefixes: append([]string(nil), defaultPrefixes...)}
	path := filepath.Join(repoRoot, ".squad", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(cfg.IDPrefixes) == 0 {
		cfg.IDPrefixes = append([]string(nil), defaultPrefixes...)
	}
	return cfg, nil
}
