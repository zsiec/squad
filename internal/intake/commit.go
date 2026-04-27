package intake

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Shape constants for a committed bundle.
const (
	ShapeItemOnly      = "item_only"
	ShapeSpecEpicItems = "spec_epic_items"
)

// Bundle is what the agent submits at intake commit time. It declares its
// own shape: items[] alone is item_only; spec + epics + items is
// spec_epic_items.
type Bundle struct {
	Spec  *SpecDraft  `json:"spec,omitempty"`
	Epics []EpicDraft `json:"epics,omitempty"`
	Items []ItemDraft `json:"items"`
}

type SpecDraft struct {
	Title         string   `json:"title"`
	Motivation    string   `json:"motivation"`
	Acceptance    []string `json:"acceptance"`
	NonGoals      []string `json:"non_goals"`
	Integration   []string `json:"integration"`
	Risks         []string `json:"risks,omitempty"`
	OpenQuestions []string `json:"open_questions,omitempty"`
}

type EpicDraft struct {
	Title        string   `json:"title"`
	Parallelism  string   `json:"parallelism"`
	Dependencies []string `json:"dependencies"`
	Status       string   `json:"status,omitempty"`
}

type ItemDraft struct {
	Title      string   `json:"title"`
	Intent     string   `json:"intent"`
	Acceptance []string `json:"acceptance"`
	Area       string   `json:"area"`
	Kind       string   `json:"kind,omitempty"`
	Effort     string   `json:"effort,omitempty"`
	// Epic is the title of the parent epic for spec_epic_items shapes;
	// ignored in item_only.
	Epic string `json:"epic,omitempty"`
}

// IntakeShapeInvalid signals a Bundle that doesn't fit any known shape:
// empty bundle, spec without epics, epics without spec, an epic with no
// items mapped to it, an item pointing at a non-existent epic, or a
// refine-mode bundle that's not exactly one item with no spec/epics.
type IntakeShapeInvalid struct {
	Reason string
}

func (e *IntakeShapeInvalid) Error() string {
	return "intake shape invalid: " + e.Reason
}

// IntakeIncomplete signals a required field that is missing or empty.
// Field is the dotted name from the checklist (e.g. "spec.motivation",
// "epic.parallelism", or "title" for item_only).
type IntakeIncomplete struct {
	Field string
}

func (e *IntakeIncomplete) Error() string {
	return "intake incomplete: missing or empty " + e.Field
}

// IntakeSlugConflict signals that a spec or epic slug is already taken
// — either by an existing DB row or by a markdown file on disk. Either
// source winning means the new artifact cannot be written without
// overwriting prior work.
type IntakeSlugConflict struct {
	Kind string
	Slug string
}

func (e *IntakeSlugConflict) Error() string {
	return fmt.Sprintf("intake slug conflict: %s slug %q already exists", e.Kind, e.Slug)
}

// CheckSlugAvailable returns nil if no spec/epic with the given slug
// exists for repoID — neither as a DB row nor as a markdown file under
// squadDir. Returns *IntakeSlugConflict if either source already holds
// the slug. kind must be "spec" or "epic"; any other value is a
// programmer error and returns a generic error.
func CheckSlugAvailable(ctx context.Context, db *sql.DB, repoID, squadDir, kind, slug string) error {
	var table, dir string
	switch kind {
	case "spec":
		table, dir = "specs", "specs"
	case "epic":
		table, dir = "epics", "epics"
	default:
		return fmt.Errorf("intake: unknown slug kind %q (want \"spec\" or \"epic\")", kind)
	}

	var n int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE repo_id = ? AND name = ?`, table)
	if err := db.QueryRowContext(ctx, query, repoID, slug).Scan(&n); err != nil {
		return fmt.Errorf("intake: probe %s table: %w", table, err)
	}
	if n > 0 {
		return &IntakeSlugConflict{Kind: kind, Slug: slug}
	}

	path := filepath.Join(squadDir, dir, slug+".md")
	if _, err := os.Stat(path); err == nil {
		return &IntakeSlugConflict{Kind: kind, Slug: slug}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("intake: stat %s: %w", path, err)
	}

	return nil
}

// Validate performs purely structural validation of bundle. It detects
// shape, walks the checklist's required fields, and asserts the
// refine-mode constraint. It does no DB or filesystem I/O.
func Validate(bundle Bundle, mode string, checklist Checklist) (string, error) {
	if mode == ModeRefine {
		if bundle.Spec != nil || len(bundle.Epics) > 0 {
			return "", &IntakeShapeInvalid{Reason: "refine mode rejects spec/epic in the bundle"}
		}
		if len(bundle.Items) != 1 {
			return "", &IntakeShapeInvalid{Reason: "refine mode requires exactly one item"}
		}
	}

	shape, err := detectShape(bundle)
	if err != nil {
		return "", err
	}

	switch shape {
	case ShapeItemOnly:
		if err := validateItemOnly(bundle, checklist); err != nil {
			return "", err
		}
	case ShapeSpecEpicItems:
		if err := validateSpecEpicItems(bundle, checklist); err != nil {
			return "", err
		}
	}
	return shape, nil
}

func detectShape(b Bundle) (string, error) {
	hasSpec := b.Spec != nil
	hasEpics := len(b.Epics) > 0
	hasItems := len(b.Items) > 0

	switch {
	case !hasItems:
		return "", &IntakeShapeInvalid{Reason: "bundle has no items"}
	case !hasSpec && !hasEpics:
		return ShapeItemOnly, nil
	case hasSpec && hasEpics:
		return ShapeSpecEpicItems, nil
	case hasSpec && !hasEpics:
		return "", &IntakeShapeInvalid{Reason: "spec present without any epics"}
	default: // !hasSpec && hasEpics
		return "", &IntakeShapeInvalid{Reason: "epics present without a spec"}
	}
}

func validateItemOnly(b Bundle, c Checklist) error {
	for _, field := range c.Required(ShapeItemOnly) {
		for _, it := range b.Items {
			if err := checkItemField(it, field, field); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSpecEpicItems(b Bundle, c Checklist) error {
	for _, field := range c.Required(ShapeSpecEpicItems) {
		kind, name, ok := splitDotted(field)
		if !ok {
			continue
		}
		switch kind {
		case "spec":
			if err := checkSpecField(b.Spec, name, field); err != nil {
				return err
			}
		case "epic":
			for _, e := range b.Epics {
				if err := checkEpicField(e, name, field); err != nil {
					return err
				}
			}
		case "item":
			for _, it := range b.Items {
				if err := checkItemField(it, name, field); err != nil {
					return err
				}
			}
		}
	}

	epicTitles := map[string]struct{}{}
	for _, e := range b.Epics {
		epicTitles[e.Title] = struct{}{}
	}
	itemsPerEpic := map[string]int{}
	for _, it := range b.Items {
		if _, ok := epicTitles[it.Epic]; !ok {
			return &IntakeShapeInvalid{Reason: "item references unknown epic " + it.Epic}
		}
		itemsPerEpic[it.Epic]++
	}
	for _, e := range b.Epics {
		if itemsPerEpic[e.Title] == 0 {
			return &IntakeShapeInvalid{Reason: "epic " + e.Title + " has no items"}
		}
	}
	return nil
}

func checkSpecField(s *SpecDraft, name, dotted string) error {
	switch name {
	case "title":
		if !slugDerivable(s.Title) {
			return &IntakeIncomplete{Field: dotted}
		}
	case "motivation":
		if strings.TrimSpace(s.Motivation) == "" {
			return &IntakeIncomplete{Field: dotted}
		}
	case "acceptance":
		if !nonEmptyBullets(s.Acceptance) {
			return &IntakeIncomplete{Field: dotted}
		}
	case "non_goals":
		if !nonEmptyBullets(s.NonGoals) {
			return &IntakeIncomplete{Field: dotted}
		}
	case "integration":
		if !nonEmptyBullets(s.Integration) {
			return &IntakeIncomplete{Field: dotted}
		}
	}
	return nil
}

func checkEpicField(e EpicDraft, name, dotted string) error {
	switch name {
	case "title":
		if !slugDerivable(e.Title) {
			return &IntakeIncomplete{Field: dotted}
		}
	case "parallelism":
		if strings.TrimSpace(e.Parallelism) == "" {
			return &IntakeIncomplete{Field: dotted}
		}
	case "dependencies":
		if e.Dependencies == nil {
			return &IntakeIncomplete{Field: dotted}
		}
	}
	return nil
}

func checkItemField(it ItemDraft, name, dotted string) error {
	switch name {
	case "title":
		if strings.TrimSpace(it.Title) == "" {
			return &IntakeIncomplete{Field: dotted}
		}
	case "intent":
		if strings.TrimSpace(it.Intent) == "" {
			return &IntakeIncomplete{Field: dotted}
		}
	case "acceptance":
		if !nonEmptyBullets(it.Acceptance) {
			return &IntakeIncomplete{Field: dotted}
		}
	case "area":
		if strings.TrimSpace(it.Area) == "" {
			return &IntakeIncomplete{Field: dotted}
		}
	}
	return nil
}

func splitDotted(s string) (kind, name string, ok bool) {
	i := strings.IndexByte(s, '.')
	if i < 0 {
		return "", s, false
	}
	return s[:i], s[i+1:], true
}

func nonEmptyBullets(bullets []string) bool {
	if len(bullets) == 0 {
		return false
	}
	for _, b := range bullets {
		if strings.TrimSpace(b) == "" {
			return false
		}
	}
	return true
}

var slugRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// slugDerivable reports whether title kebab-cases to a slug-safe
// identifier (matches ^[a-z][a-z0-9-]*$). Empty or all-symbol titles
// fail. Distinct from cmd/squad/spec_new.go's slugSafe, which validates
// an already-derived slug instead of a free-form title.
func slugDerivable(title string) bool {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlnumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return slugRe.MatchString(s)
}

var nonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)
