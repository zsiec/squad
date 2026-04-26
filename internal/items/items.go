package items

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type ACItem struct {
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

type Item struct {
	ID         string   `yaml:"id"`
	Title      string   `yaml:"title"`
	Type       string   `yaml:"type"`
	Priority   string   `yaml:"priority"`
	Area       string   `yaml:"area"`
	Status     string   `yaml:"status"`
	Estimate   string   `yaml:"estimate"`
	Risk       string   `yaml:"risk"`
	Created    string   `yaml:"created"`
	Updated    string   `yaml:"updated"`
	NotBefore  string   `yaml:"not-before"`
	BlockedBy  []string `yaml:"blocked-by"`
	RelatesTo  []string `yaml:"relates-to"`
	References []string `yaml:"references"`

	Epic             string   `yaml:"epic"`
	DependsOn        []string `yaml:"depends_on"`
	Parallel         bool     `yaml:"parallel"`
	ConflictsWith    []string `yaml:"conflicts_with"`
	EvidenceRequired []string `yaml:"evidence_required"`

	CapturedBy string `yaml:"captured_by"`
	CapturedAt int64  `yaml:"captured_at"`
	AcceptedBy string `yaml:"accepted_by"`
	AcceptedAt int64  `yaml:"accepted_at"`

	ParentSpec string `yaml:"parent_spec"`
	ParentEpic string `yaml:"parent_epic"`

	ACTotal   int      `yaml:"-"`
	ACChecked int      `yaml:"-"`
	ACItems   []ACItem `yaml:"-"`
	Body      string   `yaml:"-"`
	Path      string   `yaml:"-"`
}

func (it Item) ProgressPct() int {
	if it.ACTotal == 0 {
		return 0
	}
	return (it.ACChecked * 100) / it.ACTotal
}

var (
	// Tolerate CR\n line endings (Windows) and LF\n. UTF-8 BOM is stripped
	// before this regex runs (see Parse). Many editors prepend a BOM by
	// default; without the strip those files would be invisible to every
	// squad command.
	frontmatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)
	acHeaderRe    = regexp.MustCompile(`(?m)^##\s+Acceptance\s+criteria\s*$`)
	nextHeaderRe  = regexp.MustCompile(`(?m)^##\s+`)
	checkboxRe    = regexp.MustCompile(`^\s*[-*]\s*\[([ xX])\]\s+`)
	subBulletRe   = regexp.MustCompile(`^\s{2,}[-*]\s+`)
	// idShape constrains item IDs to PREFIX-NUMBER form. Items whose YAML
	// `id` is a non-string scalar (int, bool) or violates this shape are
	// rejected at Parse time so they don't propagate to next/status as garbage.
	idShape = regexp.MustCompile(`^[A-Z][A-Z0-9]*-\d+$`)
	utf8BOM = []byte{0xEF, 0xBB, 0xBF}
)

func Parse(path string) (Item, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Item{}, err
	}
	if len(data) >= len(utf8BOM) && string(data[:len(utf8BOM)]) == string(utf8BOM) {
		data = data[len(utf8BOM):]
	}
	match := frontmatterRe.FindSubmatch(data)
	if match == nil {
		return Item{}, fmt.Errorf("no frontmatter in %s (expected '---' fenced YAML at file start; CRLF and UTF-8 BOM are tolerated)", path)
	}
	var it Item
	if err := yaml.Unmarshal(match[1], &it); err != nil {
		return Item{}, fmt.Errorf("parse frontmatter in %s: %w", path, err)
	}
	if strings.TrimSpace(it.ID) == "" {
		return Item{}, fmt.Errorf("frontmatter in %s has no `id:` (or yaml coerced it to empty)", path)
	}
	if !idShape.MatchString(it.ID) {
		return Item{}, fmt.Errorf("frontmatter in %s has malformed id %q (expected PREFIX-NUMBER like FEAT-001)", path, it.ID)
	}
	if it.ParentEpic == "" && it.Epic != "" {
		it.ParentEpic = it.Epic
	}
	it.Path = path
	rest := data[len(match[0]):]
	it.Body = string(rest)

	hdr := acHeaderRe.FindIndex(rest)
	if hdr == nil {
		return it, nil
	}
	start := hdr[1]
	end := len(rest)
	if nxt := nextHeaderRe.FindIndex(rest[start:]); nxt != nil {
		end = start + nxt[0]
	}
	scanner := bufio.NewScanner(strings.NewReader(string(rest[start:end])))
	var cur *ACItem
	for scanner.Scan() {
		line := scanner.Text()
		if m := checkboxRe.FindStringSubmatch(line); m != nil {
			trimmed := strings.TrimSpace(checkboxRe.ReplaceAllString(line, ""))
			it.ACTotal++
			checked := strings.EqualFold(m[1], "x")
			if checked {
				it.ACChecked++
			}
			it.ACItems = append(it.ACItems, ACItem{Text: trimmed, Checked: checked})
			cur = &it.ACItems[len(it.ACItems)-1]
			continue
		}
		if cur != nil && subBulletRe.MatchString(line) {
			sub := strings.TrimSpace(subBulletRe.ReplaceAllString(line, ""))
			if sub != "" {
				cur.Text += "\n" + sub
			}
		}
	}
	return it, nil
}
