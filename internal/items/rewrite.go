package items

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

func RewriteStatus(path, newStatus string, now time.Time) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rewritten, err := rewriteFrontmatter(raw, map[string]string{
		"status":  newStatus,
		"updated": now.UTC().Format("2006-01-02"),
	})
	if err != nil {
		return fmt.Errorf("rewrite frontmatter for %s: %w", path, err)
	}
	tmp := path + ".squad.tmp"
	if err := os.WriteFile(tmp, rewritten, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func rewriteFrontmatter(raw []byte, updates map[string]string) ([]byte, error) {
	open := bytes.Index(raw, []byte("---\n"))
	if open != 0 {
		return nil, fmt.Errorf("file does not begin with frontmatter")
	}
	rest := raw[4:]
	closeIdx := bytes.Index(rest, []byte("\n---\n"))
	if closeIdx < 0 {
		return nil, fmt.Errorf("no closing --- marker for frontmatter")
	}
	frontmatter := rest[:closeIdx]
	body := rest[closeIdx+5:]

	var doc yaml.Node
	if err := yaml.Unmarshal(frontmatter, &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("frontmatter is not a YAML mapping")
	}
	mapping := doc.Content[0]

	seen := map[string]bool{}
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i].Value
		if newVal, ok := updates[key]; ok {
			mapping.Content[i+1].Value = newVal
			mapping.Content[i+1].Tag = "!!str"
			mapping.Content[i+1].Style = 0
			seen[key] = true
		}
	}
	for k, v := range updates {
		if seen[k] {
			continue
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v},
		)
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	out.WriteString("---\n")
	out.Write(body)
	return out.Bytes(), nil
}

func MoveToDone(srcPath, doneDir string) (string, error) {
	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(doneDir, filepath.Base(srcPath))
	if err := os.Rename(srcPath, dst); err != nil {
		return "", err
	}
	return dst, nil
}
