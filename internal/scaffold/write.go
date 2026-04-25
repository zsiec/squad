package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func WriteConfig(repoRoot string, d Data) error {
	return writeIfAbsent(filepath.Join(repoRoot, ".squad", "config.yaml"), "templates/config.yaml.tmpl", d)
}

func WriteStatus(repoRoot string, d Data) error {
	return writeIfAbsent(filepath.Join(repoRoot, ".squad", "STATUS.md"), "templates/status.md.tmpl", d)
}

func WriteExampleItem(repoRoot string, d Data) error {
	return writeIfAbsent(
		filepath.Join(repoRoot, ".squad", "items", "EXAMPLE-001-try-the-loop.md"),
		"templates/items/EXAMPLE-001-try-the-loop.md.tmpl",
		d,
	)
}

func WriteAgents(repoRoot string, d Data) error {
	return writeIfAbsent(filepath.Join(repoRoot, "AGENTS.md"), "templates/AGENTS.md.tmpl", d)
}

func WriteAgentsDeep(repoRoot string, d Data) error {
	return writeIfAbsent(filepath.Join(repoRoot, "docs", "agents-deep.md"), "templates/agents-deep.md.tmpl", d)
}

func writeIfAbsent(dest, tmplPath string, d Data) error {
	if _, err := os.Stat(dest); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", dest, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	raw, err := Templates.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", tmplPath, err)
	}
	rendered, err := Render(string(raw), d)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, []byte(rendered), 0o644)
}
