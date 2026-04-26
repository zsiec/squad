package installer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePluginRoot walks a plugin tree and verifies the layout matches
// what Claude Code expects: required JSON config files exist with their
// required keys, skills/ contains at least one well-formed SKILL.md.
// Returns nil if the tree is shipping-shape; otherwise a single error
// string aggregating every problem found.
func ValidatePluginRoot(root string) error {
	var errs []string

	manifest := filepath.Join(root, ".claude-plugin", "plugin.json")
	if err := assertJSONFileWithKeys(manifest, []string{"name", "version", "description"}); err != nil {
		errs = append(errs, err.Error())
	}
	mcpFile := filepath.Join(root, ".mcp.json")
	if err := assertJSONFileWithKeys(mcpFile, []string{"mcpServers"}); err != nil {
		errs = append(errs, err.Error())
	}
	hooksFile := filepath.Join(root, "hooks.json")
	if err := assertJSONFileWithKeys(hooksFile, []string{"hooks"}); err != nil {
		errs = append(errs, err.Error())
	}

	skillsDir := filepath.Join(root, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		errs = append(errs, "skills/ not readable: "+err.Error())
	} else if len(entries) == 0 {
		errs = append(errs, "skills/ is empty")
	} else {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			sf := filepath.Join(skillsDir, e.Name(), "SKILL.md")
			if _, err := os.Stat(sf); err != nil {
				errs = append(errs, fmt.Sprintf("skills/%s/SKILL.md missing", e.Name()))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func assertJSONFileWithKeys(path string, required []string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return fmt.Errorf("%s: missing required key %q", path, k)
		}
	}
	return nil
}
