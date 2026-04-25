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
	return atomicWrite(path, rewritten)
}

// atomicWrite stages content into a sibling temp file (CreateTemp's
// random-suffix pattern bounds the name length regardless of the target's
// basename — the original `path + ".squad.tmp"` overflowed the 255-byte
// FS limit when basenames pushed past ~245 bytes, see QA r6-H #1) and
// renames into place once the bytes are flushed.
func atomicWrite(path string, content []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".squad-tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	if _, err := f.Write(content); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

func rewriteFrontmatter(raw []byte, updates map[string]string) ([]byte, error) {
	// Match Parse's tolerance: strip a leading UTF-8 BOM and accept CRLF.
	// Without this, a CRLF/BOM-prefixed file that Parse() happily reads
	// fails the "begins with frontmatter" check here, so squad done leaves
	// the file behind even though the DB transaction commits (QA r6 H #2/#4).
	if len(raw) >= len(utf8BOM) && bytes.Equal(raw[:len(utf8BOM)], utf8BOM) {
		raw = raw[len(utf8BOM):]
	}
	openMarker := []byte("---\n")
	openMarkerCRLF := []byte("---\r\n")
	closeMarker := []byte("\n---\n")
	closeMarkerCRLF := []byte("\r\n---\r\n")
	openLen := len(openMarker)
	closeLen := len(closeMarker)
	if bytes.HasPrefix(raw, openMarkerCRLF) {
		openLen = len(openMarkerCRLF)
	} else if !bytes.HasPrefix(raw, openMarker) {
		return nil, fmt.Errorf("file does not begin with frontmatter")
	}
	rest := raw[openLen:]
	closeIdx := bytes.Index(rest, closeMarker)
	if idx := bytes.Index(rest, closeMarkerCRLF); idx >= 0 && (closeIdx < 0 || idx < closeIdx) {
		closeIdx = idx
		closeLen = len(closeMarkerCRLF)
	}
	if closeIdx < 0 {
		return nil, fmt.Errorf("no closing --- marker for frontmatter")
	}
	frontmatter := rest[:closeIdx]
	body := rest[closeIdx+closeLen:]

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

func EnsureBlockerSection(path, reason string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.Contains(raw, []byte("\n## Blocker")) || bytes.HasPrefix(raw, []byte("## Blocker")) {
		return nil
	}
	if !bytes.HasSuffix(raw, []byte("\n")) {
		raw = append(raw, '\n')
	}
	appended := append(raw, []byte("\n## Blocker\n"+reason+"\n")...)
	return atomicWrite(path, appended)
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
