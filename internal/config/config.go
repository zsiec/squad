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
}

type AgentConfig struct {
	// ClaimConcurrency caps how many items a single agent can hold open at
	// once. 0 means "use the default" (DefaultClaimConcurrency below).
	// Set to a large number (e.g. 100) to effectively disable.
	ClaimConcurrency int `yaml:"claim_concurrency"`
}

// DefaultClaimConcurrency is the cap applied when config doesn't override.
// Documented as 1 since Phase 6; QA round 4 surfaced that it wasn't enforced.
const DefaultClaimConcurrency = 1

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
