// Package skills hosts a tiny YAML-frontmatter parser used by the test
// scaffold under plugin/skills/*.md to validate that every shipped skill
// declares name + description + body in the expected layout.
package skills

import (
	"bytes"
	"errors"
)

var errFrontmatterNotClosed = errors.New("frontmatter delimiter not closed")

// splitFrontmatter parses a YAML frontmatter block delimited by lines of "---".
// Returns the frontmatter bytes (without delimiters) and the body bytes.
// If the file does not start with a delimiter, the entire input is returned as body.
//
// Reused at runtime by internal/mcp to rebuild skill descriptions for the MCP catalog.
func splitFrontmatter(raw []byte) (fm []byte, body []byte, err error) {
	sep := []byte("---\n")
	if !bytes.HasPrefix(raw, sep) {
		return nil, raw, nil
	}
	rest := raw[len(sep):]
	end := bytes.Index(rest, sep)
	if end < 0 {
		return nil, nil, errFrontmatterNotClosed
	}
	return rest[:end], rest[end+len(sep):], nil
}
