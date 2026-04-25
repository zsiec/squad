package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	IDPrefixes []string `yaml:"id_prefixes"`
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
