package intake

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/items"
)

// commitSpecEpicItems is the spec_epic_items branch of Commit. Layout
// mirrors commitImpl's item_only path but adds spec + epic file writes
// (cleaned up on rollback) and spec/epic row inserts inside the same tx
// the items rows go into. Slug-conflict pre-flight on the spec rolls
// the whole bundle back before any file is touched.
func commitSpecEpicItems(
	ctx context.Context,
	db *sql.DB,
	squadDir, sessionID, agentID string,
	bundle Bundle,
	ready bool,
	write itemWriter,
) (CommitResult, error) {
	sess, err := loadSession(ctx, db, sessionID)
	if err != nil {
		return CommitResult{}, err
	}
	if sess.AgentID != agentID {
		return CommitResult{}, ErrIntakeNotYours
	}
	if sess.Status != StatusOpen {
		return CommitResult{}, ErrIntakeAlreadyClosed
	}

	specSlug := slugFromTitle(bundle.Spec.Title)
	if err := CheckSlugAvailable(ctx, db, sess.RepoID, squadDir, "spec", specSlug); err != nil {
		return CommitResult{}, err
	}
	epicSlugByTitle := make(map[string]string, len(bundle.Epics))
	for _, e := range bundle.Epics {
		s := slugFromTitle(e.Title)
		if err := CheckSlugAvailable(ctx, db, sess.RepoID, squadDir, "epic", s); err != nil {
			return CommitResult{}, err
		}
		epicSlugByTitle[e.Title] = s
	}

	bundleJSON, err := json.Marshal(bundle)
	if err != nil {
		return CommitResult{}, fmt.Errorf("intake commit: marshal bundle: %w", err)
	}

	now := time.Now().Unix()

	var writtenFiles []string
	cleanup := func() {
		for _, p := range writtenFiles {
			_ = os.Remove(p)
		}
	}

	specPath := filepath.Join(squadDir, "specs", specSlug+".md")
	if err := writeSpecFile(specPath, bundle.Spec, sessionID); err != nil {
		cleanup()
		return CommitResult{}, fmt.Errorf("intake commit: write spec %s: %w", specPath, err)
	}
	writtenFiles = append(writtenFiles, specPath)

	epicPaths := make(map[string]string, len(bundle.Epics))
	for _, e := range bundle.Epics {
		slug := epicSlugByTitle[e.Title]
		ep := filepath.Join(squadDir, "epics", slug+".md")
		if err := writeEpicFile(ep, e, specSlug, sessionID); err != nil {
			cleanup()
			return CommitResult{}, fmt.Errorf("intake commit: write epic %s: %w", ep, err)
		}
		writtenFiles = append(writtenFiles, ep)
		epicPaths[e.Title] = ep
	}

	itemPaths := make([]string, 0, len(bundle.Items))
	itemEpicSlugs := make([]string, 0, len(bundle.Items))
	for _, it := range bundle.Items {
		opts := items.Options{
			Ready:           ready,
			CapturedBy:      agentID,
			ParentSpec:      specSlug,
			IntakeSessionID: sessionID,
			Epic:            epicSlugByTitle[it.Epic],
			Area:            it.Area,
			Estimate:        it.Effort,
			Intent:          it.Intent,
			Acceptance:      it.Acceptance,
		}
		prefix, ok := itemPrefixFor[it.Kind]
		if !ok {
			prefix = "FEAT"
		}
		path, werr := write(squadDir, prefix, it.Title, opts)
		if werr != nil {
			cleanup()
			return CommitResult{}, fmt.Errorf("intake commit: write item file: %w", werr)
		}
		writtenFiles = append(writtenFiles, path)
		itemPaths = append(itemPaths, path)
		itemEpicSlugs = append(itemEpicSlugs, opts.Epic)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		cleanup()
		return CommitResult{}, fmt.Errorf("intake commit: begin tx: %w", err)
	}
	rollback := func() {
		_ = tx.Rollback()
		cleanup()
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO specs (repo_id, name, title, motivation, acceptance, non_goals, integration, path, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.RepoID, specSlug, bundle.Spec.Title, bundle.Spec.Motivation,
		joinBullets(bundle.Spec.Acceptance), joinBullets(bundle.Spec.NonGoals),
		joinBullets(bundle.Spec.Integration), specPath, now,
	); err != nil {
		rollback()
		return CommitResult{}, fmt.Errorf("intake commit: insert spec row: %w", err)
	}
	for _, e := range bundle.Epics {
		slug := epicSlugByTitle[e.Title]
		status := e.Status
		if status == "" {
			status = "open"
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO epics (repo_id, name, spec, status, parallelism, path, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			sess.RepoID, slug, specSlug, status, e.Parallelism, epicPaths[e.Title], now,
		); err != nil {
			rollback()
			return CommitResult{}, fmt.Errorf("intake commit: insert epic row %s: %w", slug, err)
		}
	}

	ids := make([]string, 0, len(itemPaths))
	for i, p := range itemPaths {
		parsed, perr := items.Parse(p)
		if perr != nil {
			rollback()
			return CommitResult{}, fmt.Errorf("intake commit: parse %s: %w", p, perr)
		}
		// Belt-and-braces: enforce row hierarchy from the bundle even if
		// frontmatter parsing dropped a field. PersistOne reads these.
		parsed.ParentSpec = specSlug
		parsed.Epic = itemEpicSlugs[i]
		if perr := items.PersistOne(ctx, tx, sess.RepoID, parsed, false, now); perr != nil {
			rollback()
			return CommitResult{}, fmt.Errorf("intake commit: persist row %s: %w", parsed.ID, perr)
		}
		if _, perr := tx.ExecContext(ctx,
			`UPDATE items SET intake_session_id=? WHERE repo_id=? AND item_id=?`,
			sessionID, sess.RepoID, parsed.ID,
		); perr != nil {
			rollback()
			return CommitResult{}, fmt.Errorf("intake commit: link session on %s: %w", parsed.ID, perr)
		}
		ids = append(ids, parsed.ID)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE intake_sessions
		SET status='committed', shape=?, bundle_json=?, committed_at=?, updated_at=?
		WHERE id=?
	`, ShapeSpecEpicItems, string(bundleJSON), now, now, sessionID); err != nil {
		rollback()
		return CommitResult{}, fmt.Errorf("intake commit: mark session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		// Same orphan-files-over-orphan-rows trade-off as commitImpl.
		cleanup()
		return CommitResult{}, fmt.Errorf("intake commit: tx commit: %w", err)
	}

	allPaths := append([]string{specPath}, append(values(epicPaths), itemPaths...)...)
	return CommitResult{Shape: ShapeSpecEpicItems, ItemIDs: ids, Paths: allPaths}, nil
}

func writeSpecFile(path string, s *SpecDraft, sessionID string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf(specStubTemplate,
		yamlInlineSpec(s.Title), yamlBlockOrEmpty(s.Motivation),
		yamlBulletList(s.Acceptance), yamlBulletList(s.NonGoals),
		yamlBulletList(s.Integration), yamlBulletList(s.Risks),
		yamlBulletList(s.OpenQuestions), sessionID,
	)
	return os.WriteFile(path, []byte(body), 0o644)
}

func writeEpicFile(path string, e EpicDraft, specSlug, sessionID string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	status := e.Status
	if status == "" {
		status = "open"
	}
	body := fmt.Sprintf(epicStubTemplate,
		yamlInlineSpec(e.Title), specSlug, status,
		yamlBlockOrEmpty(e.Parallelism), yamlStringList(e.Dependencies), sessionID,
	)
	return os.WriteFile(path, []byte(body), 0o644)
}

const specStubTemplate = `---
title: %s
motivation: %s
acceptance:%s
non_goals:%s
integration:%s
risks:%s
open_questions:%s
intake_session: %s
---

## Background
(Filled in during intake.)
`

const epicStubTemplate = `---
title: %s
spec: %s
status: %s
parallelism: %s
dependencies:%s
intake_session: %s
---

## Notes
(Filled in during intake.)
`

func yamlInlineSpec(s string) string {
	if strings.ContainsAny(s, ":\n\"'") || strings.HasPrefix(s, "-") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func yamlBlockOrEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "\"\""
	}
	if strings.Contains(s, "\n") {
		return "|\n  " + strings.ReplaceAll(s, "\n", "\n  ")
	}
	return yamlInlineSpec(s)
}

func yamlBulletList(items []string) string {
	if len(items) == 0 {
		return " []"
	}
	var b strings.Builder
	for _, it := range items {
		b.WriteString("\n  - ")
		b.WriteString(yamlInlineSpec(it))
	}
	return b.String()
}

func yamlStringList(items []string) string {
	return yamlBulletList(items)
}

func joinBullets(bullets []string) string {
	return strings.Join(bullets, "\n")
}

var slugReplaceRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugFromTitle kebab-cases a title into the [a-z][a-z0-9-]* shape the
// rest of squad uses for spec/epic identifiers. Validate already enforced
// slugDerivable on the title, so the result here is guaranteed non-empty
// and conformant.
func slugFromTitle(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = slugReplaceRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// values returns map values in iteration order. Used to build the Paths
// slice in CommitResult; ordering across epics doesn't matter to callers.
func values(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
