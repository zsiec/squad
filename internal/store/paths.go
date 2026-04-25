package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func Home() (string, error) {
	if env := os.Getenv("SQUAD_HOME"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	return filepath.Join(home, ".squad"), nil
}

func EnsureHome() error {
	home, err := Home()
	if err != nil {
		return err
	}
	for _, sub := range []string{"", "archive", "backups"} {
		if err := os.MkdirAll(filepath.Join(home, sub), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", sub, err)
		}
	}
	return nil
}

func DBPath() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "global.db"), nil
}
