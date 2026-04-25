package scaffold

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	startMarker = "<!-- squad-managed:start -->"
	endMarker   = "<!-- squad-managed:end -->"
)

type MergeChoice int

const (
	ChoiceBottom MergeChoice = iota
	ChoiceTop
	ChoiceAbort
)

var ErrMergeAborted = errors.New("CLAUDE.md merge aborted by user")

func MergeCLAUDE(repoRoot string, d Data, choice MergeChoice) error {
	dest := filepath.Join(repoRoot, "CLAUDE.md")

	rawTmpl, err := Templates.ReadFile("templates/claude.md.fragment.tmpl")
	if err != nil {
		return err
	}
	rendered, err := Render(string(rawTmpl), d)
	if err != nil {
		return err
	}
	rendered = strings.TrimRight(rendered, "\n")

	current, err := os.ReadFile(dest)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return os.WriteFile(dest, []byte(rendered+"\n"), 0o644)
	case err != nil:
		return fmt.Errorf("read CLAUDE.md: %w", err)
	}

	cur := string(current)
	if strings.Contains(cur, startMarker) && strings.Contains(cur, endMarker) {
		merged, err := replaceBlock(cur, rendered)
		if err != nil {
			return err
		}
		if merged == cur {
			return nil
		}
		return os.WriteFile(dest, []byte(merged), 0o644)
	}

	switch choice {
	case ChoiceAbort:
		return ErrMergeAborted
	case ChoiceTop:
		return os.WriteFile(dest, []byte(rendered+"\n"+cur), 0o644)
	case ChoiceBottom:
		sep := "\n\n"
		if strings.HasSuffix(cur, "\n") {
			sep = "\n"
		}
		return os.WriteFile(dest, []byte(cur+sep+rendered+"\n"), 0o644)
	default:
		return fmt.Errorf("unknown merge choice: %d", choice)
	}
}

func replaceBlock(content, replacement string) (string, error) {
	startIdx := strings.Index(content, startMarker)
	endIdx := strings.Index(content, endMarker)
	if startIdx < 0 || endIdx < 0 || endIdx < startIdx {
		return "", fmt.Errorf("malformed managed-block markers")
	}
	endIdx += len(endMarker)
	return content[:startIdx] + replacement + content[endIdx:], nil
}
